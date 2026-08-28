#!/usr/bin/env python3
"""
把 Edge headless 渲染出的多尺寸 PNG 打包成标准 ICO（PNG-in-ICO，Vista+ 支持）。
不依赖 Pillow，仅使用 Python 标准库。
"""
import os
import struct
import sys

SIZES = [16, 24, 32, 48, 64, 128, 256]
SRC_DIR = os.environ.get("ICON_SRC_DIR", os.path.join(os.environ.get("TEMP", "/tmp")))
OUT_ICO = os.path.join(os.path.dirname(__file__), "icon.ico")
OUT_APPICON = os.path.join(os.path.dirname(os.path.dirname(__file__)), "appicon.png")


def pack_ico(png_files):
    """生成含 PNG 条目的 .ico 文件字节。"""
    count = len(png_files)
    # 图标文件头：Reserved(2) + Type(1=icon) + Count(2)
    header = struct.pack("<HHH", 0, 1, count)
    # 目录项每项 16 字节，目录后紧跟 PNG 数据
    offset = 6 + 16 * count
    directory = b""
    data = b""
    for size, path in png_files:
        png = open(path, "rb").read()
        # ICO 目录里 0 表示 256，规范不支持 >256（我们最高 256）
        dir_w = size if size < 256 else 0
        dir_h = dir_w
        # PNG-in-ICO：Planes=1, BitCount=32，颜色数 0，保留 0
        entry = struct.pack(
            "<BBBBHHII",
            dir_w,       # Width
            dir_h,       # Height
            0,           # Colors (0 if >256)
            0,           # Reserved
            1,           # Planes
            32,          # Bit count
            len(png),    # Size in bytes
            offset,      # Offset
        )
        directory += entry
        data += png
        offset += len(png)
    return header + directory + data


def main():
    png_files = []
    for size in SIZES:
        src = os.path.join(SRC_DIR, f"icon_{size}.png")
        if not os.path.exists(src):
            print(f"[ERR] 缺少 {src}", file=sys.stderr)
            sys.exit(1)
        png_files.append((size, src))

    # 生成 .ico
    ico = pack_ico(png_files)
    with open(OUT_ICO, "wb") as f:
        f.write(ico)
    print(f"[OK] 生成 {OUT_ICO} ({len(ico)} bytes, {count} 个尺寸)")

    # 同时更新 build/appicon.png（512 → 256 缩放没有必要，直接复制 512 作为商店展示图）
    src512 = os.path.join(SRC_DIR, "icon_512.png")
    with open(src512, "rb") as s, open(OUT_APPICON, "wb") as d:
        d.write(s.read())
    print(f"[OK] 更新 {OUT_APPICON}")

    # 简单校验 ICO：打印目录
    with open(OUT_ICO, "rb") as f:
        f.read(6)
        print("目录项:")
        for _ in range(count):
            b = f.read(16)
            w, h, col, _, planes, bpp, size, off = struct.unpack("<BBBBHHII", b)
            w = 256 if w == 0 else w
            h = 256 if h == 0 else h
            print(f"  {w}x{h}  {bpp}-bit  {size} bytes  @0x{off:x}")


if __name__ == "__main__":
    count = len(SIZES)
    main()
