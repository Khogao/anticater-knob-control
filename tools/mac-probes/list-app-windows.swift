import CoreGraphics
import Foundation
let list = CGWindowListCopyWindowInfo([.optionAll], kCGNullWindowID) as? [[String: Any]] ?? []
for w in list where (w[kCGWindowOwnerName as String] as? String ?? "").contains("Knob") || (w[kCGWindowOwnerName as String] as? String ?? "").contains("ANTICATER") {
    print(w[kCGWindowOwnerName as String] ?? "", w[kCGWindowLayer as String] ?? "", w[kCGWindowIsOnscreen as String] ?? "offscreen", w[kCGWindowBounds as String] ?? "")
}
