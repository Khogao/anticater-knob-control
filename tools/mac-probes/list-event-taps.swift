import CoreGraphics
import Foundation
var count: UInt32 = 0
CGGetEventTapList(0, nil, &count)
var taps = [CGEventTapInformation](repeating: CGEventTapInformation(), count: Int(count))
CGGetEventTapList(count, &taps, &count)
for t in taps.prefix(Int(count)) {
    print("pid \(t.tappingProcess) loc \(t.tapPoint) options \(t.options) enabled \(t.enabled) mask 0x\(String(t.eventsOfInterest, radix: 16)) minLatency \(t.minUsecLatency) avg \(t.avgUsecLatency)")
}
