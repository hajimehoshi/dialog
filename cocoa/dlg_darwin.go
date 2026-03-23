package cocoa

import (
	"errors"
	"runtime"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

const (
	nsAlertFirstButtonReturn                = 1000
	nsApplicationActivationPolicyProhibited = 2
	nsApplicationActivationPolicyAccessory  = 1
)

var (
	class_NSAlert       objc.Class
	class_NSApplication objc.Class
	class_NSArray       objc.Class
	class_NSImage       objc.Class
	class_NSOpenPanel   objc.Class
	class_NSSavePanel   objc.Class
	class_NSString      objc.Class
	class_NSThread      objc.Class
	class_NSURL         objc.Class
)

var (
	sel_URL                     objc.SEL
	sel_URLs                    objc.SEL
	sel_UTF8String              objc.SEL
	sel_activationPolicy        objc.SEL
	sel_addButtonWithTitle      objc.SEL
	sel_alloc                   objc.SEL
	sel_arrayWithObjects_count  objc.SEL
	sel_fileURLWithPath         objc.SEL
	sel_imageNamed              objc.SEL
	sel_init                    objc.SEL
	sel_isMainThread            objc.SEL
	sel_objectAtIndex           objc.SEL
	sel_openPanel               objc.SEL
	sel_path                    objc.SEL
	sel_runModal                objc.SEL
	sel_savePanel               objc.SEL
	sel_setActivationPolicy     objc.SEL
	sel_setAllowedFileTypes     objc.SEL
	sel_setAllowsOtherFileTypes objc.SEL
	sel_setCanChooseDirectories objc.SEL
	sel_setCanChooseFiles       objc.SEL
	sel_setDirectoryURL         objc.SEL
	sel_setFloatingPanel        objc.SEL
	sel_setIcon                 objc.SEL
	sel_setMessageText          objc.SEL
	sel_setNameFieldStringValue objc.SEL
	sel_setTitle                objc.SEL
	sel_sharedApplication       objc.SEL
	sel_stringWithUTF8String    objc.SEL
	sel_window                  objc.SEL
)

var (
	nsImageNameCaution objc.ID
	nsImageNameInfo    objc.ID
)

var (
	dispatchSyncF func(queue uintptr, context uintptr, work uintptr)
	dispatchMainQ uintptr
	mainThreadCB  uintptr
	mainThreadMu  sync.Mutex
	mainThreadFn  func()
)

func init() {
	cocoaLib, err := purego.Dlopen("/System/Library/Frameworks/Cocoa.framework/Cocoa", purego.RTLD_GLOBAL|purego.RTLD_LAZY)
	if err != nil {
		panic(err)
	}

	libSystem, err := purego.Dlopen("/usr/lib/libSystem.B.dylib", purego.RTLD_LAZY)
	if err != nil {
		panic(err)
	}
	purego.RegisterLibFunc(&dispatchSyncF, libSystem, "dispatch_sync_f")
	dispatchMainQ, err = purego.Dlsym(libSystem, "_dispatch_main_q")
	if err != nil {
		panic(err)
	}

	mainThreadCB = purego.NewCallback(func(context uintptr) {
		mainThreadFn()
	})

	class_NSAlert = objc.GetClass("NSAlert")
	class_NSApplication = objc.GetClass("NSApplication")
	class_NSArray = objc.GetClass("NSArray")
	class_NSImage = objc.GetClass("NSImage")
	class_NSOpenPanel = objc.GetClass("NSOpenPanel")
	class_NSSavePanel = objc.GetClass("NSSavePanel")
	class_NSString = objc.GetClass("NSString")
	class_NSThread = objc.GetClass("NSThread")
	class_NSURL = objc.GetClass("NSURL")

	sel_URL = objc.RegisterName("URL")
	sel_URLs = objc.RegisterName("URLs")
	sel_UTF8String = objc.RegisterName("UTF8String")
	sel_activationPolicy = objc.RegisterName("activationPolicy")
	sel_addButtonWithTitle = objc.RegisterName("addButtonWithTitle:")
	sel_alloc = objc.RegisterName("alloc")
	sel_arrayWithObjects_count = objc.RegisterName("arrayWithObjects:count:")
	sel_fileURLWithPath = objc.RegisterName("fileURLWithPath:")
	sel_imageNamed = objc.RegisterName("imageNamed:")
	sel_init = objc.RegisterName("init")
	sel_isMainThread = objc.RegisterName("isMainThread")
	sel_objectAtIndex = objc.RegisterName("objectAtIndex:")
	sel_openPanel = objc.RegisterName("openPanel")
	sel_path = objc.RegisterName("path")
	sel_runModal = objc.RegisterName("runModal")
	sel_savePanel = objc.RegisterName("savePanel")
	sel_setActivationPolicy = objc.RegisterName("setActivationPolicy:")
	sel_setAllowedFileTypes = objc.RegisterName("setAllowedFileTypes:")
	sel_setAllowsOtherFileTypes = objc.RegisterName("setAllowsOtherFileTypes:")
	sel_setCanChooseDirectories = objc.RegisterName("setCanChooseDirectories:")
	sel_setCanChooseFiles = objc.RegisterName("setCanChooseFiles:")
	sel_setDirectoryURL = objc.RegisterName("setDirectoryURL:")
	sel_setFloatingPanel = objc.RegisterName("setFloatingPanel:")
	sel_setIcon = objc.RegisterName("setIcon:")
	sel_setMessageText = objc.RegisterName("setMessageText:")
	sel_setNameFieldStringValue = objc.RegisterName("setNameFieldStringValue:")
	sel_setTitle = objc.RegisterName("setTitle:")
	sel_sharedApplication = objc.RegisterName("sharedApplication")
	sel_stringWithUTF8String = objc.RegisterName("stringWithUTF8String:")
	sel_window = objc.RegisterName("window")

	cautionSym, err := purego.Dlsym(cocoaLib, "NSImageNameCaution")
	if err != nil {
		panic(err)
	}
	nsImageNameCaution = *(*objc.ID)(unsafe.Pointer(cautionSym))

	infoSym, err := purego.Dlsym(cocoaLib, "NSImageNameInfo")
	if err != nil {
		panic(err)
	}
	nsImageNameInfo = *(*objc.ID)(unsafe.Pointer(infoSym))
}

func toNSString(s string) objc.ID {
	return objc.ID(class_NSString).Send(sel_stringWithUTF8String, s)
}

func toGoString(nsStr objc.ID) string {
	return objc.Send[string](nsStr, sel_UTF8String)
}

func checkActivationPolicy() {
	nsApp := objc.ID(class_NSApplication).Send(sel_sharedApplication)
	policy := objc.Send[int](nsApp, sel_activationPolicy)
	if policy == nsApplicationActivationPolicyProhibited {
		nsApp.Send(sel_setActivationPolicy, nsApplicationActivationPolicyAccessory)
	}
}

func runOnMainThread(fn func()) {
	if objc.Send[bool](objc.ID(class_NSThread), sel_isMainThread) {
		fn()
		return
	}
	mainThreadMu.Lock()
	defer mainThreadMu.Unlock()
	mainThreadFn = fn
	dispatchSyncF(dispatchMainQ, 0, mainThreadCB)
}

const (
	alertYesNo = iota
	alertError
	alertInfo
)

func alertDlg(msg, title string, style int) bool {
	var result bool
	runOnMainThread(func() {
		alert := objc.ID(class_NSAlert).Send(sel_alloc).Send(sel_init)
		if title != "" {
			alert.Send(sel_window).Send(sel_setTitle, toNSString(title))
		}
		alert.Send(sel_setMessageText, toNSString(msg))
		switch style {
		case alertYesNo:
			alert.Send(sel_addButtonWithTitle, toNSString("Yes"))
			alert.Send(sel_addButtonWithTitle, toNSString("No"))
		case alertError:
			alert.Send(sel_setIcon, objc.ID(class_NSImage).Send(sel_imageNamed, nsImageNameCaution))
			alert.Send(sel_addButtonWithTitle, toNSString("OK"))
		case alertInfo:
			alert.Send(sel_setIcon, objc.ID(class_NSImage).Send(sel_imageNamed, nsImageNameInfo))
			alert.Send(sel_addButtonWithTitle, toNSString("OK"))
		}
		checkActivationPolicy()
		result = objc.Send[int](alert, sel_runModal) == nsAlertFirstButtonReturn
	})
	return result
}

func YesNoDlg(msg, title string) bool {
	return alertDlg(msg, title, alertYesNo)
}

func InfoDlg(msg, title string) {
	alertDlg(msg, title, alertInfo)
}

func ErrorDlg(msg, title string) {
	alertDlg(msg, title, alertError)
}

const (
	loadDlg = iota
	saveDlg
	dirDlg
)

func FileDlg(save bool, title string, exts []string, relaxExt bool, startDir string, filename string) (string, error) {
	mode := loadDlg
	if save {
		mode = saveDlg
	}
	return fileDlg(mode, title, exts, relaxExt, startDir, filename)
}

func DirDlg(title string, startDir string) (string, error) {
	return fileDlg(dirDlg, title, nil, false, startDir, "")
}

func fileDlg(mode int, title string, exts []string, relaxExt bool, startDir, filename string) (string, error) {
	var resultPath string
	var resultErr error

	runOnMainThread(func() {
		var panel objc.ID
		if mode == saveDlg {
			panel = objc.ID(class_NSSavePanel).Send(sel_savePanel)
		} else {
			panel = objc.ID(class_NSOpenPanel).Send(sel_openPanel)
			if mode == dirDlg {
				panel.Send(sel_setCanChooseDirectories, true)
				panel.Send(sel_setCanChooseFiles, false)
			}
		}

		panel.Send(sel_setFloatingPanel, true)

		if title != "" {
			panel.Send(sel_setTitle, toNSString(title))
		}
		if len(exts) > 0 {
			nsExts := make([]objc.ID, len(exts))
			for i, ext := range exts {
				nsExts[i] = toNSString(ext)
			}
			array := objc.ID(class_NSArray).Send(sel_arrayWithObjects_count,
				unsafe.Pointer(&nsExts[0]), uint(len(nsExts)))
			runtime.KeepAlive(nsExts)
			panel.Send(sel_setAllowedFileTypes, array)
		}
		if relaxExt {
			panel.Send(sel_setAllowsOtherFileTypes, true)
		}
		if startDir != "" {
			url := objc.ID(class_NSURL).Send(sel_fileURLWithPath, toNSString(startDir))
			panel.Send(sel_setDirectoryURL, url)
		}
		if filename != "" {
			panel.Send(sel_setNameFieldStringValue, toNSString(filename))
		}

		checkActivationPolicy()

		if objc.Send[int](panel, sel_runModal) == 0 {
			return
		}

		var url objc.ID
		if mode == saveDlg {
			url = panel.Send(sel_URL)
		} else {
			url = panel.Send(sel_URLs).Send(sel_objectAtIndex, uint(0))
		}
		pathNS := url.Send(sel_path)
		if pathNS == 0 {
			resultErr = errors.New("failed to get file-system representation for selected URL")
			return
		}
		resultPath = toGoString(pathNS)
	})

	return resultPath, resultErr
}
