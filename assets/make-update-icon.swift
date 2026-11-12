// Draws assets/update-icon.png, the update window's icon (design.md 4.5)
// until Gocast has an app icon: the About page's placeholder, SF Symbols'
// command.square, 48 in a 64 DIP square (128 px). One image serves both
// appearances, so it is system gray (#8E8E93), which stands out on the
// window's light and dark backgrounds (3.3:1 and 5:1 or more).
//
//	swift assets/make-update-icon.swift assets/update-icon.png

import AppKit
let px = 128, pt = 64.0
let cfg = NSImage.SymbolConfiguration(pointSize: 40, weight: .regular)
let img = NSImage(systemSymbolName: "command.square", accessibilityDescription: nil)!.withSymbolConfiguration(cfg)!
let rep = NSBitmapImageRep(bitmapDataPlanes: nil, pixelsWide: px, pixelsHigh: px, bitsPerSample: 8, samplesPerPixel: 4, hasAlpha: true, isPlanar: false, colorSpaceName: .deviceRGB, bytesPerRow: 0, bitsPerPixel: 0)!
rep.size = NSSize(width: pt, height: pt)
NSGraphicsContext.saveGraphicsState()
NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: rep)
let s = img.size
let r = NSRect(x: (pt - s.width)/2, y: (pt - s.height)/2, width: s.width, height: s.height)
img.draw(in: r)
NSColor(srgbRed: 0x8E/255.0, green: 0x8E/255.0, blue: 0x93/255.0, alpha: 1).set()
r.fill(using: .sourceAtop)
NSGraphicsContext.restoreGraphicsState()
try! rep.representation(using: .png, properties: [:])!.write(to: URL(fileURLWithPath: CommandLine.arguments[1]))
print(s)
