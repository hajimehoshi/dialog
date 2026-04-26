package dialog

import (
	"fmt"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

type WinDlgError int

func (e WinDlgError) Error() string {
	return fmt.Sprintf("CommDlgExtendedError: %#x", int(e))
}

func err() error {
	e := _CommDlgExtendedError()
	if e == 0 {
		return ErrCancelled
	}
	return WinDlgError(e)
}

func (b *MsgBuilder) yesNo() bool {
	r, _ := _MessageBox(0, b.Msg, firstOf(b.Dlg.Title, "Confirm?"), _MB_YESNO)
	return r == _IDYES
}

func (b *MsgBuilder) info() {
	_, _ = _MessageBox(0, b.Msg, firstOf(b.Dlg.Title, "Information"), _MB_OK|_MB_ICONINFORMATION)
}

func (b *MsgBuilder) error() {
	_, _ = _MessageBox(0, b.Msg, firstOf(b.Dlg.Title, "Error"), _MB_OK|_MB_ICONERROR)
}

type filedlg struct {
	opf     *_OPENFILENAME
	fileBuf []uint16
}

func (d filedlg) Filename() string {
	return windows.UTF16ToString(d.fileBuf)
}

func (b *FileBuilder) load() (string, error) {
	d := openfile(_OFN_FILEMUSTEXIST|_OFN_NOCHANGEDIR, b)
	if _GetOpenFileName(d.opf) {
		return d.Filename(), nil
	}
	return "", err()
}

func (b *FileBuilder) save() (string, error) {
	d := openfile(_OFN_OVERWRITEPROMPT|_OFN_NOCHANGEDIR, b)
	if _GetSaveFileName(d.opf) {
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
	d.opf = &_OPENFILENAME{
		Owner: _GetForegroundWindow(),
	}

	startFile, err := windows.UTF16FromString(b.StartFile)
	if err != nil {
		panic(fmt.Sprintf("dialog: UTF16FromString failed: %v", err))
	}
	// The buffer pointed to by opf.File receives the selected file path, so it
	// must be large enough to hold any path the user might pick, not just
	// StartFile. Use a 32Ki-wchar buffer to accommodate long paths.
	const fileBufLen = 32 * 1024
	d.fileBuf = make([]uint16, fileBufLen)
	copy(d.fileBuf, startFile)
	d.opf.File = &d.fileBuf[0]
	d.opf.MaxFile = uint32(len(d.fileBuf))
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
	bi *_BROWSEINFO
}

func callbackDefaultDir(hwnd windows.HWND, msg uint, lParam, lpData uintptr) int {
	if msg == _BFFM_INITIALIZED {
		_ = _SendMessage(hwnd, _BFFM_SETSELECTION, _TRUE, lpData)
	}
	return 0
}

func selectdir(b *DirectoryBuilder) (d dirdlg) {
	// Use GetForegroundWindow to get the current window.
	// GetActiveWindow returns the window that belongs to the current thread.
	// TODO: Use GetActiveWindow for predictable behavior.
	d.bi = &_BROWSEINFO{
		Flags: _BIF_RETURNONLYFSDIRS | _BIF_NEWDIALOGSTYLE,
		Owner: _GetForegroundWindow(),
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
	res := _SHBrowseForFolder(d.bi)
	if res == 0 {
		return "", ErrCancelled
	}
	return _SHGetPathFromIDList(res)
}
