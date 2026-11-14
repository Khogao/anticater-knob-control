// Listen-only probe: logs key and system-defined (media key) events with timing.
import AppKit
import CoreGraphics
import Foundation

let seconds = Double(CommandLine.arguments.dropFirst().first ?? "5") ?? 5
setvbuf(stdout, nil, _IOLBF, 0)
let mask: CGEventMask = (1 << 10) | (1 << 11) | (1 << 12) | (1 << 14)
let cb: CGEventTapCallBack = { _, type, event, refcon in
    let t = String(format: "%.3f", ProcessInfo.processInfo.systemUptime) + (refcon == nil ? " PRE " : " POST")
    let tag = event.getIntegerValueField(.eventSourceUserData)
    let pid = event.getIntegerValueField(.eventSourceUnixProcessID)
    switch type.rawValue {
    case 10, 11:
        print("\(t) \(type.rawValue == 10 ? "keyDown" : "keyUp  ") keycode=\(event.getIntegerValueField(.keyboardEventKeycode)) flags=0x\(String(event.flags.rawValue, radix: 16)) srcpid=\(pid) tag=\(tag)")
    case 14:
        if let ns = NSEvent(cgEvent: event), ns.subtype.rawValue == 8 {
            let key = (ns.data1 & 0xFFFF0000) >> 16
            let down = ((ns.data1 & 0xFF00) >> 8) == 0xA
            print("\(t) sysdef  nxkey=\(key) \(down ? "down" : "up") srcpid=\(pid) tag=\(tag)")
        }
    default: break
    }
    return Unmanaged.passUnretained(event)
}
for (place, ref) in [(CGEventTapPlacement.headInsertEventTap, UnsafeMutableRawPointer(bitPattern: 0)), (CGEventTapPlacement.tailAppendEventTap, UnsafeMutableRawPointer(bitPattern: 1))] {
    guard let tap = CGEvent.tapCreate(tap: .cgSessionEventTap, place: place, options: .listenOnly,
                                      eventsOfInterest: mask, callback: cb, userInfo: ref) else {
        print("TAP_FAILED"); exit(1)
    }
    CFRunLoopAddSource(CFRunLoopGetCurrent(), CFMachPortCreateRunLoopSource(nil, tap, 0), .commonModes)
    CGEvent.tapEnable(tap: tap, enable: true)
}
print("probe listening for \(seconds)s")
DispatchQueue.main.asyncAfter(deadline: .now() + seconds) { print("probe done"); exit(0) }
CFRunLoopRun()
