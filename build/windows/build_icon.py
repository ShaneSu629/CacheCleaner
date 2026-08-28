#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
CacheCleaner 图标构建脚本（纯标准库，无需 Pillow / cairosvg）。

流程：
  icon.svg --(Edge/Chrome headless 渲染 512)--> base.png --(面积平均降采样)--> 多尺寸 PNG --> icon.ico

为什么只渲染一次 512 再降采样：
  Edge headless 会按 SVG 固有尺寸(width/height=512)渲染，若把 --window-size 设成 16/32，
  截图只会截到画布左上角的透明区域，导致小尺寸图标全白/全透明。
  因此固定用 512x512 窗口渲染，小尺寸一律由脚本降采样得到。

用法：
  python build_icon.py                 # 默认读同目录 icon.svg，输出 icon.ico + ../appicon.png
  python build_icon.py --check         # 只做像素校验，不写文件
"""
import os
import sys
import struct
import zlib
import shutil
import subprocess
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
SRC_SVG = os.path.join(HERE, 'icon.svg')
OUT_ICO = os.path.join(HERE, 'icon.ico')
OUT_APPICON = os.path.abspath(os.path.join(HERE, '..', 'appicon.png'))
SIZES = [16, 24, 32, 48, 64, 128, 256]
BASE = 512

# --------------------------------------------------------------------------
# PNG 解码
# --------------------------------------------------------------------------
def read_png(path):
    with open(path, 'rb') as f:
        data = f.read()
    if data[:8] != b'\x89PNG\r\n\x1a\n':
        raise SystemExit('不是合法的 PNG: %s' % path)
    pos, idat = 8, b''
    w = h = bd = ct = None
    plte = trns = None
    while pos + 8 <= len(data):
        ln = struct.unpack('>I', data[pos:pos + 4])[0]
        typ = data[pos + 4:pos + 8]
        body = data[pos + 8:pos + 8 + ln]
        if typ == b'IHDR':
            w, h, bd, ct, _comp, _filt, inter = struct.unpack('>IIBBBBB', body)
            if inter:
                raise SystemExit('不支持隔行扫描 PNG')
        elif typ == b'IDAT':
            idat += body
        elif typ == b'PLTE':
            plte = body
        elif typ == b'tRNS':
            trns = body
        elif typ == b'IEND':
            break
        pos += 12 + ln
    if bd != 8:
        raise SystemExit('仅支持 8bit PNG，当前 bitdepth=%s' % bd)
    nch = {0: 1, 2: 3, 3: 1, 4: 2, 6: 4}[ct]
    raw = zlib.decompress(idat)
    stride = w * nch
    out = bytearray(w * h * nch)
    prev = bytearray(stride)
    p = 0
    for y in range(h):
        ft = raw[p]; p += 1
        line = bytearray(raw[p:p + stride]); p += stride
        if ft == 1:
            for i in range(nch, stride):
                line[i] = (line[i] + line[i - nch]) & 0xFF
        elif ft == 2:
            for i in range(stride):
                line[i] = (line[i] + prev[i]) & 0xFF
        elif ft == 3:
            for i in range(stride):
                a = line[i - nch] if i >= nch else 0
                line[i] = (line[i] + ((a + prev[i]) >> 1)) & 0xFF
        elif ft == 4:
            for i in range(stride):
                a = line[i - nch] if i >= nch else 0
                b = prev[i]
                c = prev[i - nch] if i >= nch else 0
                pa, pb, pc = abs(b - c), abs(a - c), abs(a + b - 2 * c)
                pr = a if (pa <= pb and pa <= pc) else (b if pb <= pc else c)
                line[i] = (line[i] + pr) & 0xFF
        out[y * stride:(y + 1) * stride] = line
        prev = line
    return {'w': w, 'h': h, 'ct': ct, 'nch': nch, 'px': bytes(out),
            'plte': plte, 'trns': trns}


def to_rgba(img):
    """任意颜色类型统一转成 RGBA（长度 w*h*4）。"""
    w, h, ct, px = img['w'], img['h'], img['ct'], img['px']
    out = bytearray(w * h * 4)
    if ct == 6:
        out[:] = px
    elif ct == 2:
        for i in range(w * h):
            out[i * 4:i * 4 + 3] = px[i * 3:i * 3 + 3]
            out[i * 4 + 3] = 255
    elif ct == 3:
        plte, trns = img['plte'], img['trns']
        for i in range(w * h):
            v = px[i]
            out[i * 4:i * 4 + 3] = plte[v * 3:v * 3 + 3]
            out[i * 4 + 3] = trns[v] if trns and v < len(trns) else 255
    elif ct == 0:
        for i in range(w * h):
            v = px[i]
            out[i * 4] = out[i * 4 + 1] = out[i * 4 + 2] = v
            out[i * 4 + 3] = 255
    elif ct == 4:
        for i in range(w * h):
            v = px[i]
            out[i * 4] = out[i * 4 + 1] = out[i * 4 + 2] = v
            out[i * 4 + 3] = px[i * 2 + 1]
    else:
        raise SystemExit('不支持的颜色类型 %s' % ct)
    return out


# --------------------------------------------------------------------------
# 降采样：预乘 alpha + 面积平均（box filter）
# --------------------------------------------------------------------------
def _spans(start, end, nmax):
    """把浮点区间 [start,end) 切分为整数像素及其覆盖权重。"""
    res = []
    i = int(start)
    while i < end and i < nmax:
        w = min(i + 1, end) - max(i, start)
        if w > 1e-9:
            res.append((i, w))
        i += 1
    if not res and nmax > 0:
        res.append((min(int(start), nmax - 1), end - start))
    return res


def resize(rgba, sw, sh, dw, dh):
    """按源像素覆盖面积加权平均；alpha 预乘，避免半透明边缘出现黑边。"""
    out = bytearray(dw * dh * 4)
    fx, fy = sw / dw, sh / dh
    for dy in range(dh):
        yspans = _spans(dy * fy, (dy + 1) * fy, sh)
        for dx in range(dw):
            xspans = _spans(dx * fx, (dx + 1) * fx, sw)
            ar = ag = ab = aa = tw = 0.0
            for y, wyy in yspans:
                row = y * sw * 4
                for x, wxx in xspans:
                    w = wxx * wyy
                    i = row + x * 4
                    a = rgba[i + 3]
                    ar += rgba[i] * a * w
                    ag += rgba[i + 1] * a * w
                    ab += rgba[i + 2] * a * w
                    aa += a * w
                    tw += w
            o = (dy * dw + dx) * 4
            if aa > 1e-9 and tw > 1e-9:
                out[o]     = max(0, min(255, int(ar / aa + 0.5)))
                out[o + 1] = max(0, min(255, int(ag / aa + 0.5)))
                out[o + 2] = max(0, min(255, int(ab / aa + 0.5)))
                out[o + 3] = max(0, min(255, int(aa / tw + 0.5)))
    return out


# --------------------------------------------------------------------------
# PNG 编码 / ICO 打包
# --------------------------------------------------------------------------
def write_png(rgba, w, h):
    stride = w * 4
    raw = bytearray()
    for y in range(h):
        raw.append(0)                       # filter type 0 (None)
        raw += rgba[y * stride:(y + 1) * stride]

    def chunk(typ, body):
        return (struct.pack('>I', len(body)) + typ + body
                + struct.pack('>I', zlib.crc32(typ + body) & 0xFFFFFFFF))

    return (b'\x89PNG\r\n\x1a\n'
            + chunk(b'IHDR', struct.pack('>IIBBBBB', w, h, 8, 6, 0, 0, 0))
            + chunk(b'IDAT', zlib.compress(bytes(raw), 9))
            + chunk(b'IEND', b''))


def pack_ico(images):
    """images: [(size, png_bytes)]。BMP 头中 256 用 0 表示。"""
    n = len(images)
    offset = 6 + 16 * n
    entries, blobs = b'', b''
    for size, png in images:
        b = 0 if size >= 256 else size
        entries += struct.pack('<BBBBHHII', b, b, 0, 0, 1, 32, len(png), offset)
        blobs += png
        offset += len(png)
    return struct.pack('<HHH', 0, 1, n) + entries + blobs


# --------------------------------------------------------------------------
# 渲染（Edge / Chrome headless）
# --------------------------------------------------------------------------
def find_browser():
    cands = [
        r'C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe',
        r'C:\Program Files\Microsoft\Edge\Application\msedge.exe',
        r'C:\Program Files (x86)\Google\Chrome\Application\chrome.exe',
        r'C:\Program Files\Google\Chrome\Application\chrome.exe',
    ]
    for c in cands:
        if os.path.exists(c):
            return c
    for name in ('msedge', 'chrome'):
        p = shutil.which(name)
        if p:
            return p
    raise SystemExit('未找到 Edge / Chrome，无法渲染 SVG')


def render_base_png(svg_path, out_png, size=BASE):
    """用固定 512x512 视口渲染 SVG，避免小窗口截图截到透明角。"""
    with open(svg_path, 'r', encoding='utf-8') as f:
        svg = f.read()
    html = ('<!DOCTYPE html><html><head><meta charset="utf-8"><style>'
            'html,body{margin:0;padding:0;width:%dpx;height:%dpx;overflow:hidden;'
            'background:transparent}svg{display:block;width:%dpx;height:%dpx}'
            '</style></head><body>%s</body></html>' % (size, size, size, size, svg))
    tmpdir = tempfile.mkdtemp(prefix='ccicon_')
    html_path = os.path.join(tmpdir, 'icon.html')
    with open(html_path, 'w', encoding='utf-8') as f:
        f.write(html)
    # 独立 user-data-dir，避免占用用户正在运行的浏览器实例
    profile = os.path.join(tmpdir, 'profile')
    cmd = [
        find_browser(),
        '--headless=new',
        '--disable-gpu',
        '--hide-scrollbars',
        '--disable-extensions',
        '--force-device-scale-factor=1',
        '--default-background-color=00000000',
        '--user-data-dir=%s' % profile,
        '--window-size=%d,%d' % (size, size),
        '--screenshot=%s' % out_png,
        'file:///' + html_path.replace('\\', '/'),
    ]
    r = subprocess.run(cmd, capture_output=True, timeout=120)
    if not os.path.exists(out_png):
        raise SystemExit('渲染失败：\n%s' % r.stderr.decode('utf-8', 'ignore'))
    return out_png


# --------------------------------------------------------------------------
# 校验
# --------------------------------------------------------------------------
def opaque_ratio(rgba, w, h):
    return sum(1 for i in range(w * h) if rgba[i * 4 + 3] > 16) / (w * h)


def shape_preview(rgba, w, h, cols=40):
    step = max(1, w // cols)
    lines = []
    for y in range(0, h, step * 2):
        s = ''
        for x in range(0, w, step):
            i = (y * w + x) * 4
            r, g, b, a = rgba[i], rgba[i + 1], rgba[i + 2], rgba[i + 3]
            if a < 40:
                s += ' '
            else:
                lum = r * 0.299 + g * 0.587 + b * 0.114
                s += '#' if lum > 200 else ('+' if lum > 110 else ('.' if lum > 60 else ','))
        lines.append(s)
    return '\n'.join(lines)


def main():
    check_only = '--check' in sys.argv
    tmpdir = tempfile.mkdtemp(prefix='ccicon_')
    base_png = os.path.join(tmpdir, 'base.png')
    print('渲染 %s -> %dpx ...' % (os.path.basename(SRC_SVG), BASE))
    render_base_png(SRC_SVG, base_png, BASE)

    img = read_png(base_png)
    base = to_rgba(img)
    print('源图 %dx%d，非透明 %.2f%%\n'
          % (img['w'], img['h'], opaque_ratio(base, img['w'], img['h']) * 100))

    images, made, bad = [], {}, []
    for s in SIZES:
        cur = resize(base, img['w'], img['h'], s, s)
        made[s] = cur
        ratio = opaque_ratio(cur, s, s)
        flag = ''
        if ratio < 0.90:                       # 圆角磁贴理论占比约 95%
            flag = '  <== 异常，疑似空白/裁切'
            bad.append(s)
        print('  icon_%-4d %4dpx  非透明 %6.2f%%%s' % (s, s, ratio * 100, flag))
        images.append((s, write_png(cur, s, s)))

    if check_only:
        for s in (32, 16):
            print('\n--- %dpx 形状预览（#=白 +=亮蓝 .=深蓝 空=透明）---' % s)
            print(shape_preview(made[s], s, s))
        shutil.rmtree(tmpdir, ignore_errors=True)
        return 1 if bad else 0

    if bad:
        shutil.rmtree(tmpdir, ignore_errors=True)
        raise SystemExit('尺寸 %s 渲染异常，已中止，未覆盖 icon.ico' % bad)

    with open(OUT_ICO, 'wb') as f:
        f.write(pack_ico(images))
    with open(OUT_APPICON, 'wb') as f:
        f.write(write_png(base, img['w'], img['h']))
    print('\n已生成 %s  (%.1f KB, %d 档)' % (OUT_ICO, os.path.getsize(OUT_ICO) / 1024, len(images)))
    print('已生成 %s  (%.1f KB)' % (OUT_APPICON, os.path.getsize(OUT_APPICON) / 1024))

    for s in (32, 16):
        print('\n--- %dpx 形状预览（#=白 +=亮蓝 .=深蓝 空=透明）---' % s)
        print(shape_preview(made[s], s, s))
    shutil.rmtree(tmpdir, ignore_errors=True)
    return 0


if __name__ == '__main__':
    sys.exit(main())
