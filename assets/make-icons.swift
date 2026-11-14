// Renders assets/logo.svg into the app icons:
//   assets/AppIcon.icns          (macOS, picked up by build-gui.sh)
//   windows/AnticaterKnob.ico    (Windows, loaded by AnticaterKnob.exe from its own folder)
//
// Usage (macOS):  swift assets/make-icons.swift assets/logo.svg [RRGGBB]
// The optional colour replaces the fill of the <g id="logo"> group for this run only.
import AppKit
import Foundation

let args = CommandLine.arguments
guard args.count >= 2 else { print("usage: make-icons.swift logo.svg [RRGGBB]"); exit(2) }
let svgURL = URL(fileURLWithPath: args[1])
let root = svgURL.deletingLastPathComponent().deletingLastPathComponent()
var svg = try! String(contentsOf: svgURL, encoding: .utf8)
if args.count >= 3 {
    let colour = args[2].trimmingCharacters(in: CharacterSet(charactersIn: "#"))
    guard colour.count == 6, UInt32(colour, radix: 16) != nil else { print("colour must be RRGGBB"); exit(2) }
    svg = svg.replacingOccurrences(of: #"(<g id="logo" fill=")#[0-9A-Fa-f]{6}"#, with: "$1#\(colour)", options: .regularExpression)
}
guard let image = NSImage(data: Data(svg.utf8)) else { print("could not read the SVG"); exit(1) }

// Straight (non-premultiplied) RGBA pixels of the SVG at size x size.
func render(_ size: Int) -> [UInt8] {
    var buf = [UInt8](repeating: 0, count: size * size * 4)
    let ctx = CGContext(data: &buf, width: size, height: size, bitsPerComponent: 8, bytesPerRow: size * 4,
                        space: CGColorSpace(name: CGColorSpace.sRGB)!, bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue)!
    ctx.interpolationQuality = .high
    NSGraphicsContext.saveGraphicsState()
    NSGraphicsContext.current = NSGraphicsContext(cgContext: ctx, flipped: false)
    image.draw(in: NSRect(x: 0, y: 0, width: size, height: size), from: .zero, operation: .copy, fraction: 1)
    NSGraphicsContext.restoreGraphicsState()
    for i in stride(from: 0, to: buf.count, by: 4) {
        let a = Int(buf[i + 3])
        if a > 0 && a < 255 { for c in 0..<3 { buf[i + c] = UInt8(min(255, Int(buf[i + c]) * 255 / a)) } }
    }
    return buf
}

func png(_ rgba: [UInt8], _ size: Int) -> Data {
    var pixels = rgba
    let rep = pixels.withUnsafeMutableBufferPointer { p -> NSBitmapImageRep in
        var plane: UnsafeMutablePointer<UInt8>? = p.baseAddress
        let r = NSBitmapImageRep(bitmapDataPlanes: &plane, pixelsWide: size, pixelsHigh: size, bitsPerSample: 8,
                                 samplesPerPixel: 4, hasAlpha: true, isPlanar: false, colorSpaceName: .deviceRGB,
                                 bitmapFormat: .alphaNonpremultiplied, bytesPerRow: size * 4, bitsPerPixel: 32)!
        return r
    }
    return rep.representation(using: .png, properties: [:])!
}

extension Data {
    mutating func le16(_ v: Int) { append(UInt8(v & 0xFF)); append(UInt8((v >> 8) & 0xFF)) }
    mutating func le32(_ v: Int) { le16(v & 0xFFFF); le16((v >> 16) & 0xFFFF) }
}

// --- macOS .icns via iconutil -------------------------------------------------
let iconset = FileManager.default.temporaryDirectory.appendingPathComponent("knob-\(getpid()).iconset")
try? FileManager.default.removeItem(at: iconset)
try! FileManager.default.createDirectory(at: iconset, withIntermediateDirectories: true)
for (points, scale) in [(16, 1), (16, 2), (32, 1), (32, 2), (128, 1), (128, 2), (256, 1), (256, 2), (512, 1), (512, 2)] {
    let size = points * scale
    let name = "icon_\(points)x\(points)\(scale == 2 ? "@2x" : "").png"
    try! png(render(size), size).write(to: iconset.appendingPathComponent(name))
}
let icns = root.appendingPathComponent("assets/AppIcon.icns")
let tool = Process()
tool.executableURL = URL(fileURLWithPath: "/usr/bin/iconutil")
tool.arguments = ["-c", "icns", iconset.path, "-o", icns.path]
try! tool.run(); tool.waitUntilExit()
try? FileManager.default.removeItem(at: iconset)
print(tool.terminationStatus == 0 ? "wrote \(icns.path)" : "iconutil failed")

// --- Windows .ico ---------------------------------------------------------------
// Small sizes are stored as 32-bit BMP images (read by every Windows version);
// 256 is stored as PNG, as Windows expects.
var images: [(size: Int, data: Data)] = []
for size in [16, 20, 24, 32, 40, 48, 64] {
    let rgba = render(size)
    var d = Data()
    d.le32(40); d.le32(size); d.le32(size * 2); d.le16(1); d.le16(32); d.le32(0)
    d.le32(size * size * 4); d.le32(0); d.le32(0); d.le32(0); d.le32(0)
    for y in stride(from: size - 1, through: 0, by: -1) {          // bottom-up BGRA
        for x in 0..<size {
            let i = (y * size + x) * 4
            d.append(contentsOf: [rgba[i + 2], rgba[i + 1], rgba[i], rgba[i + 3]])
        }
    }
    d.append(Data(count: ((size + 31) / 32) * 4 * size))             // empty AND mask
    images.append((size, d))
}
images.append((256, png(render(256), 256)))
var ico = Data()
ico.le16(0); ico.le16(1); ico.le16(images.count)
var offset = 6 + 16 * images.count
for (size, data) in images {
    ico.append(UInt8(size == 256 ? 0 : size)); ico.append(UInt8(size == 256 ? 0 : size))
    ico.append(0); ico.append(0); ico.le16(1); ico.le16(32); ico.le32(data.count); ico.le32(offset)
    offset += data.count
}
for (_, data) in images { ico.append(data) }
let icoURL = root.appendingPathComponent("windows/AnticaterKnob.ico")
try! ico.write(to: icoURL)
print("wrote \(icoURL.path) (\(images.count) sizes)")
