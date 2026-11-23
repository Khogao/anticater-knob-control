package main

import (
	"os/exec"
	"syscall"
	"unsafe"
)

// Only one Anticater Knob may run on the machine at a time: two copies would
// both act on every knob turn and fight over the knob's lights.
//
// The lock is a named mutex, held until the process ends (Windows releases it
// even when the process is killed or crashes, so a dead copy never blocks a
// new one). Two names are taken:
//
//   - Global\AnticaterKnobWindows covers every Windows session on the machine
//     (another user signed in, fast user switching, Remote Desktop).
//   - Local\AnticaterKnobWindows is the name older builds used, so a new build
//     also refuses to start next to an old one still running from another folder.
//
// Creating the mutex is atomic, so copies started at the same moment (autostart
// plus a double-click) cannot both win.

const (
	instanceGlobal = `Global\AnticaterKnobWindows`
	instanceLocal  = `Local\AnticaterKnobWindows`

	errorAccessDenied = 5
)

var (
	pCreateJobObjectW         = kernel32.NewProc("CreateJobObjectW")
	pSetInformationJobObject  = kernel32.NewProc("SetInformationJobObject")
	pAssignProcessToJobObject = kernel32.NewProc("AssignProcessToJobObject")
	pIsHungAppWindow          = user32.NewProc("IsHungAppWindow")
	pIsProcessInJob           = kernel32.NewProc("IsProcessInJob")

	instanceHandles []uintptr // kept open for the life of the process
	helperJob       uintptr
)

// takeMutex creates the named mutex. It reports false when another process
// already holds the name. ERROR_ACCESS_DENIED means the mutex exists but was
// created by another user, which also counts as already running.
func takeMutex(name string) bool {
	h, _, lastErr := pCreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(u16(name))))
	errno, _ := lastErr.(syscall.Errno)
	if h == 0 {
		return errno != errorAccessDenied && errno != errorAlreadyExists
	}
	if errno == errorAlreadyExists {
		pCloseHandle.Call(h)
		return false
	}
	instanceHandles = append(instanceHandles, h)
	return true
}

// claimSingleInstance reports whether this process is the only copy.
func claimSingleInstance() bool {
	return takeMutex(instanceGlobal) && takeMutex(instanceLocal)
}

// handOffToRunningCopy is called when another copy holds the lock: it asks
// that copy to show its settings window, or explains why it cannot.
func handOffToRunningCopy() {
	setLanguage("")
	other, _, _ := pFindWindowW.Call(uintptr(unsafe.Pointer(u16(mainClassName))), 0)
	if other == 0 {
		// Running in another Windows session, or still starting up.
		messageBox("Anticater Knob", L("Anticater Knob đang chạy rồi (có thể ở một phiên đăng nhập khác). Không mở thêm bản thứ hai.",
			"Anticater Knob is already running (possibly in another sign-in session). A second copy was not started."))
		return
	}
	if hung, _, _ := pIsHungAppWindow.Call(other); hung != 0 {
		messageBox("Anticater Knob", L("Anticater Knob đang chạy nhưng không phản hồi. Mở Task Manager, kết thúc AnticaterKnob.exe rồi mở lại.",
			"Anticater Knob is running but not responding. End AnticaterKnob.exe in Task Manager, then start it again."))
		return
	}
	pPostMessageW.Call(other, wmShowUI, 0, 0)
}

// jobExtendedLimit is JOBOBJECT_EXTENDED_LIMIT_INFORMATION (144 bytes on amd64).
type jobExtendedLimit struct {
	PerProcessUserTimeLimit int64
	PerJobUserTimeLimit     int64
	LimitFlags              uint32
	MinimumWorkingSetSize   uintptr
	MaximumWorkingSetSize   uintptr
	ActiveProcessLimit      uint32
	Affinity                uintptr
	PriorityClass           uint32
	SchedulingClass         uint32
	IoCounters              [6]uint64
	ProcessMemoryLimit      uintptr
	JobMemoryLimit          uintptr
	PeakProcessMemoryUsed   uintptr
	PeakJobMemoryUsed       uintptr
}

const (
	jobObjectExtendedLimitInformation = 9
	jobObjectLimitKillOnJobClose      = 0x2000
	processSetQuota                   = 0x0100
	processTerminate                  = 0x0001
)

// windowInAppJob reports whether a window belongs to a process this copy of the
// app started (the settings browser window), as opposed to another app's or
// the person's own Edge window that happens to carry the same title.
func windowInAppJob(hwnd uintptr) bool {
	if helperJob == 0 {
		return false
	}
	var pid uint32
	pGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid == 0 {
		return false
	}
	h, _, _ := pOpenProcess.Call(processQueryLimited, 0, uintptr(pid))
	if h == 0 {
		return false
	}
	defer pCloseHandle.Call(h)
	var in int32
	ok, _, _ := pIsProcessInJob.Call(h, helperJob, uintptr(unsafe.Pointer(&in)))
	return ok != 0 && in != 0
}

// tieToApp puts a helper process (the LED script, the settings browser) in a job that Windows closes
// when this app ends for any reason, killing the helper with it. Without this a
// helper left over from a killed copy keeps the knob's light service open.
func tieToApp(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	if helperJob == 0 {
		j, _, _ := pCreateJobObjectW.Call(0, 0)
		if j == 0 {
			return
		}
		info := jobExtendedLimit{LimitFlags: jobObjectLimitKillOnJobClose}
		if ok, _, _ := pSetInformationJobObject.Call(j, jobObjectExtendedLimitInformation,
			uintptr(unsafe.Pointer(&info)), unsafe.Sizeof(info)); ok == 0 {
			pCloseHandle.Call(j)
			return
		}
		helperJob = j // never closed: closing it is what kills the helpers
	}
	h, _, _ := pOpenProcess.Call(processSetQuota|processTerminate, 0, uintptr(cmd.Process.Pid))
	if h == 0 {
		return
	}
	if ok, _, err := pAssignProcessToJobObject.Call(helperJob, h); ok == 0 {
		logf("led helper: could not tie to app: %v", err)
	}
	pCloseHandle.Call(h)
}
