from PIL import Image, ImageDraw, ImageFont, ImageFilter
import os, sys
# Renders the share image (web/public/og-image.png, 1200x630) and the app
# icons (apple-touch-icon, icon-192/512) from the AppLogo mark and the landing
# dashboard capture. Run after capture-web.mjs, from anywhere:
#   python scripts/screenshots/og-image.py
# Needs Pillow and the Segoe UI fonts (Windows); bump `?v=` on ogImage in
# web/app/components/LandingPage.vue whenever the image changes.
pub = os.path.join(os.path.dirname(os.path.abspath(__file__)), '..', '..', 'web', 'public')
S = 4  # supersampling

def lerp(a, b, t): return tuple(int(a[i] + (b[i]-a[i])*t) for i in range(3))
def hexc(h): h=h.lstrip('#'); return tuple(int(h[i:i+2],16) for i in (0,2,4))

def gradient(w, h, c1, c2, diag=True):
    g = Image.new('RGB', (w, h))
    px = g.load()
    for y in range(h):
        for x in range(w):
            t = ((x/w + y/h)/2) if diag else y/h
            px[x, y] = lerp(c1, c2, t)
    return g

def card(w):
    """AppLogo mark (without the CC lettering), width w, aspect 26:36, untilted."""
    h = int(w*36/26)
    im = Image.new('RGBA', (w, h), (0,0,0,0))
    shell = gradient(w, h, hexc('#4c1d95'), hexc('#2f2159')).convert('RGBA')
    m = Image.new('L', (w, h), 0); ImageDraw.Draw(m).rounded_rectangle([0,0,w-1,h-1], int(w*.15), fill=255)
    im.paste(shell, (0,0), m)
    ins = int(w*.08)
    d = ImageDraw.Draw(im)
    d.rounded_rectangle([ins, ins, w-1-ins, h-1-ins], int(w*.12), fill=hexc('#e9e4fb'))
    pad = int(w*.06)
    ax0, ay0, ax1, ay1 = ins+pad, ins+pad, w-1-ins-pad, int(h*.70)
    art = gradient(ax1-ax0, ay1-ay0, hexc('#8b5cf6'), hexc('#a855f7')).convert('RGBA')
    am = Image.new('L', art.size, 0); ImageDraw.Draw(am).rounded_rectangle([0,0,art.size[0]-1,art.size[1]-1], int(art.size[0]*.15), fill=255)
    im.paste(art, (ax0, ay0), am)
    return im

def icon(size, bg):
    big = size*S
    im = Image.new('RGBA', (big, big), bg)
    c = card(int(big*0.50)).rotate(6, resample=Image.BICUBIC, expand=True)
    im.alpha_composite(c, ((big-c.width)//2, (big-c.height)//2))
    return im.resize((size, size), Image.LANCZOS)

bg = hexc('#0a0714') + (255,)
icon(180, bg).convert('RGB').save(os.path.join(pub, 'apple-touch-icon.png'), optimize=True)
icon(192, bg).convert('RGB').save(os.path.join(pub, 'icon-192.png'), optimize=True)
icon(512, bg).convert('RGB').save(os.path.join(pub, 'icon-512.png'), optimize=True)

# Open Graph image, 1200x630
W, H = 1200, 630
og = Image.new('RGB', (W, H), hexc('#050308'))
glow = Image.new('RGB', (W, H), hexc('#050308'))
gd = ImageDraw.Draw(glow)
gd.ellipse([-300, -420, 900, 420], fill=hexc('#1e1b4b'))
gd.ellipse([700, 250, 1500, 900], fill=hexc('#2a1460'))
og = glow.filter(ImageFilter.GaussianBlur(160))

shot = Image.open(os.path.join(pub, 'landing', 'dashboard.webp')).convert('RGB')
sw = 720; sh = int(shot.height*sw/shot.width)
shot = shot.resize((sw, sh), Image.LANCZOS)
frame = Image.new('RGBA', (sw+16, sh+16), (0,0,0,0))
ImageDraw.Draw(frame).rounded_rectangle([0,0,sw+15,sh+15], 22, fill=(36, 30, 58, 255))
sm = Image.new('L', (sw, sh), 0); ImageDraw.Draw(sm).rounded_rectangle([0,0,sw-1,sh-1], 16, fill=255)
frame.paste(shot, (8, 8), sm)
og = og.convert('RGBA')
og.alpha_composite(frame, (560, 150))

c = card(84).rotate(6, resample=Image.BICUBIC, expand=True)
og.alpha_composite(c, (72, 60))
d = ImageDraw.Draw(og)
F = 'C:/Windows/Fonts/'
bold = ImageFont.truetype(F+'segoeuib.ttf', 60)
semi = ImageFont.truetype(F+'seguisb.ttf', 30)
reg = ImageFont.truetype(F+'segoeui.ttf', 26)
d.text((72, 196), 'Tapeando', font=bold, fill=hexc('#f1f0f6'))
d.text((72, 262), 'Cartones', font=bold, fill=hexc('#c4b5fd'))
d.text((74, 356), 'Commander', font=semi, fill=hexc('#f1f0f6'))
d.text((74, 400), 'Magic: The Gathering', font=reg, fill=hexc('#a9a3c2'))
# Call to action: a pill styled like the landing's primary button, plus the domain.
btn = ImageFont.truetype(F+'segoeuib.ttf', 28)
label = 'Empieza gratis  →'
bw = int(d.textlength(label, font=btn)) + 64
bx, by, bh = 72, 466, 64
pill = gradient(bw, bh, hexc('#8b5cf6'), hexc('#a855f7'), diag=False)
pill = Image.new('RGB', (bw, bh)); pp = pill.load()
for x in range(bw):
    col = lerp(hexc('#8b5cf6'), hexc('#a855f7'), x/bw)
    for y in range(bh): pp[x, y] = col
pm = Image.new('L', (bw, bh), 0); ImageDraw.Draw(pm).rounded_rectangle([0,0,bw-1,bh-1], bh//2, fill=255)
og.paste(pill, (bx, by), pm)
d.text((bx+32, by+bh//2), label, font=btn, fill=hexc('#0a0714'), anchor='lm')
d.text((bx+4, by+bh+38), 'tapeandocartones.es', font=reg, fill=hexc('#c4b5fd'), anchor='lm')
og.convert('RGB').save(os.path.join(pub, 'og-image.png'), optimize=True)
