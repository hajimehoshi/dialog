package dialog

import (
	"fmt"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"github.com/TheTitanrain/w32"
	"golang.org/x/sys/windows"
)

type WinDlgError int

func (e WinDlgError) Error() string {
	return fmt.Sprintf("CommDlgExtendedError: %#x", int(e))
}

func err() error {
	e := w32.CommDlgExtendedError()
	if e == 0 {
		return ErrCancelled
	}
	return WinDlgError(e)
}

func (b *MsgBuilder) yesNo() bool {
	r := w32.MessageBox(w32.HWND(0), b.Msg, firstOf(b.Dlg.Title, "Confirm?"), w32.MB_YESNO)
	return r == w32.IDYES
}

func (b *MsgBuilder) info() {
	w32.MessageBox(w32.HWND(0), b.Msg, firstOf(b.Dlg.Title, "Information"), w32.MB_OK|w32.MB_ICONINFORMATION)
}

func (b *MsgBuilder) error() {
	w32.MessageBox(w32.HWND(0), b.Msg, firstOf(b.Dlg.Title, "Error"), w32.MB_OK|w32.MB_ICONERROR)
}

type filedlg struct {
	filename string
	opf      *w32.OPENFILENAME
}

func (d filedlg) Filename() string {
	return d.filename
}

func (b *FileBuilder) load() (string, error) {
	d := openfile(w32.OFN_FILEMUSTEXIST|w32.OFN_NOCHANGEDIR, b)
	if w32.GetOpenFileName(d.opf) {
		return d.Filename(), nil
	}
	return "", err()
}

func (b *FileBuilder) save() (string, error) {
	d := openfile(w32.OFN_OVERWRITEPROMPT|w32.OFN_NOCHANGEDIR, b)
	if w32.GetSaveFileName(d.opf) {
		return d.Filename(), nil
	}
	return "", err()
}

func utf16FromStringWithoutNullTermination(s string) []uint16 {
	return utf16.Encode([]rune(s))
}

func openfile(flags uint32, b *FileBuilder) (d filedlg) {
	// Use GetForegroundWindow to get the current window.
	// GetActiveWindow returns the window that belongs to the current thread.
	// TODO: Use GetActiveWindow for predictable behavior.
	d.opf = &w32.OPENFILENAME{
		Owner: w32.HWND(_GetForegroundWindow()),
	}

	d.filename = b.StartFile
	startFile, err := windows.UTF16FromString(b.StartFile)
	if err != nil {
		panic(fmt.Sprintf("dialog: UTF16FromString failed: %v", err))
	}
	d.opf.File = &startFile[0]
	d.opf.MaxFile = uint32(len(startFile))
	d.opf.Flags = flags

	d.opf.StructSize = uint32(unsafe.Sizeof(*d.opf))

	if b.StartDir != "" {
		initialDir, err := windows.UTF16PtrFromString(b.StartDir)
		if err != nil {
			panic(fmt.Sprintf("dialog: UTF16PtrFromString failed: %v", err))
		}
		d.opf.InitialDir = initialDir
	}

	if b.Dlg.Title != "" {
		title, err := windows.UTF16PtrFromString(b.Dlg.Title)
		if err != nil {
			panic(fmt.Sprintf("dialog: UTF16PtrFromString failed: %v", err))
		}
		d.opf.Title = title
	}

	var filters []uint16
	for _, filt := range b.Filters {
		// Build UTF-16 string of form "Music File\0*.mp3;*.ogg;*.wav;\0".
		filters = append(filters, utf16FromStringWithoutNullTermination(filt.Desc)...)
		filters = append(filters, 0)
		for _, ext := range filt.Extensions {
			s := fmt.Sprintf("*.%s;", ext)
			filters = append(filters, utf16FromStringWithoutNullTermination(s)...)
			filters = append(filters, 0)
		}
		filters = append(filters, 0)
	}
	if len(filters) > 0 {
		// Add two extra NUL chars to terminate the list.
		filters = append(filters, 0, 0)
		d.opf.Filter = &filters[0]
	}
	return d
}

type dirdlg struct {
	bi *w32.BROWSEINFO
}

const (
	bffm_INITIALIZED     = 1
	bffm_SELCHANGED      = 2
	bffm_VALIDATEFAILEDA = 3
	bffm_VALIDATEFAILEDW = 4
	bffm_SETSTATUSTEXTA  = (w32.WM_USER + 100)
	bffm_SETSTATUSTEXTW  = (w32.WM_USER + 104)
	bffm_ENABLEOK        = (w32.WM_USER + 101)
	bffm_SETSELECTIONA   = (w32.WM_USER + 102)
	bffm_SETSELECTIONW   = (w32.WM_USER + 103)
	bffm_SETOKTEXT       = (w32.WM_USER + 105)
	bffm_SETEXPANDED     = (w32.WM_USER + 106)
	bffm_SETSTATUSTEXT   = bffm_SETSTATUSTEXTW
	bffm_SETSELECTION    = bffm_SETSELECTIONW
	bffm_VALIDATEFAILED  = bffm_VALIDATEFAILEDW
)

func callbackDefaultDir(hwnd w32.HWND, msg uint, lParam, lpData uintptr) int {
	if msg == bffm_INITIALIZED {
		_ = w32.SendMessage(hwnd, bffm_SETSELECTION, w32.TRUE, lpData)
	}
	return 0
}

func selectdir(b *DirectoryBuilder) (d dirdlg) {
	// Use GetForegroundWindow to get the current window.
	// GetActiveWindow returns the window that belongs to the current thread.
	// TODO: Use GetActiveWindow for predictable behavior.
	d.bi = &w32.BROWSEINFO{
		Flags: w32.BIF_RETURNONLYFSDIRS | w32.BIF_NEWDIALOGSTYLE,
		Owner: w32.HWND(_GetForegroundWindow()),
	}
	if b.Dlg.Title != "" {
		d.bi.Title, _ = syscall.UTF16PtrFromString(b.Dlg.Title)
	}
	if b.StartDir != "" {
		s16, _ := syscall.UTF16PtrFromString(b.StartDir)
		d.bi.LParam = uintptr(unsafe.Pointer(s16))
		d.bi.CallbackFunc = syscall.NewCallback(callbackDefaultDir)
	}
	return d
}

func (b *DirectoryBuilder) browse() (string, error) {
	d := selectdir(b)
	res := w32.SHBrowseForFolder(d.bi)
	if res == 0 {
		return "", ErrCancelled
	}
	return w32.SHGetPathFromIDList(res), nil
}
