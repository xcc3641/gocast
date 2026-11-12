// Draws assets/tray.png: SF Symbols' command.square, 18 DIP (36 px), black
// on transparent, for the tray (a template image macOS tints).
//
//	swift assets/make-tray-icon.swift assets/tray.png

import AppKit
let px = 36
let cfg = NSImage.SymbolConfiguration(pointSize: 15, weight: .regular)
let img = NSImage(systemSymbolName: "command.square", accessibilityDescription: nil)!.withSymbolConfiguration(cfg)!
let rep = NSBitmapImageRep(bitmapDataPlanes: nil, pixelsWide: px, pixelsHigh: px, bitsPerSample: 8, samplesPerPixel: 4, hasAlpha: true, isPlanar: false, colorSpaceName: .deviceRGB, bytesPerRow: 0, bitsPerPixel: 0)!
rep.size = NSSize(width: 18, height: 18)
NSGraphicsContext.saveGraphicsState()
NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: rep)
let s = img.size
let r = NSRect(x: (18 - s.width)/2, y: (18 - s.height)/2, width: s.width, height: s.height)
img.draw(in: r)
NSColor.black.set(); r.fill(using: .sourceAtop)
NSGraphicsContext.restoreGraphicsState()
try! rep.representation(using: .png, properties: [:])!.write(to: URL(fileURLWithPath: CommandLine.arguments[1]))
