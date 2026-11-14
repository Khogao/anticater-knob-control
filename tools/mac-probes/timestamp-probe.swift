// Compares timestamps of the knob's HID reports with the media-key events macOS makes from them.
import AppKit
import CoreGraphics
import Foundation
import IOKit.hid

setvbuf(stdout, nil, _IOLBF, 0)
var tb = mach_timebase_info_data_t(); mach_timebase_info(&tb)
var reports: [UInt64] = []
let cb: CGEventTapCallBack = { _, type, event, _ in
    if type.rawValue == 14, event.getIntegerValueField(.eventSourceUserData) == 0,
       let ns = NSEvent(cgEvent: event), ns.subtype.rawValue == 8 {
        let key = (ns.data1 & 0xFFFF0000) >> 16
        let t = event.timestamp
        let nearest = reports.map { Int64(bitPattern: $0) - Int64(bitPattern: t) }.min { abs($0) < abs($1) }
        print("key nx=\(key) ts=\(t / 1_000_000)ms nearestReportDelta=\(nearest.map { String($0 / 1_000_000) + "ms" } ?? "none") reportsSeen=\(reports.count)")
    }
    return Unmanaged.passUnretained(event)
}
guard let tap = CGEvent.tapCreate(tap: .cgSessionEventTap, place: .headInsertEventTap, options: .listenOnly,
                                  eventsOfInterest: CGEventMask(1) << 14, callback: cb, userInfo: nil) else { print("TAP_FAILED"); exit(1) }
CFRunLoopAddSource(CFRunLoopGetCurrent(), CFMachPortCreateRunLoopSource(nil, tap, 0), .commonModes)
let m = IOHIDManagerCreate(kCFAllocatorDefault, 0)
IOHIDManagerSetDeviceMatching(m, [kIOHIDVendorIDKey: 0x05ac, kIOHIDProductIDKey: 0x022c] as CFDictionary)
IOHIDManagerRegisterInputValueCallback(m, { _, _, _, value in
    reports.append(IOHIDValueGetTimeStamp(value) * UInt64(tb.numer) / UInt64(tb.denom))
}, nil)
IOHIDManagerScheduleWithRunLoop(m, CFRunLoopGetCurrent(), CFRunLoopMode.defaultMode.rawValue)
let r = IOHIDManagerOpen(m, 0)
print("hid open: 0x\(String(UInt32(bitPattern: r), radix: 16)) devices: \((IOHIDManagerCopyDevices(m) as? Set<IOHIDDevice>)?.count ?? 0)")
DispatchQueue.main.asyncAfter(deadline: .now() + 25) { print("done reports=\(reports.count)"); exit(0) }
CFRunLoopRun()
