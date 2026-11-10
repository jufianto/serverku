"""Portable Serverku original-vector brand kit builder."""
from pathlib import Path
import hashlib
import json
import shutil
import subprocess
from string import Template
from PIL import Image, ImageDraw, ImageFont
from fontTools.ttLib import TTFont
from fontTools.pens.svgPathPen import SVGPathPen
from reportlab.pdfgen import canvas
from reportlab.pdfbase import pdfmetrics
from reportlab.pdfbase.ttfonts import TTFont as RLFont
from reportlab.lib.colors import HexColor
from reportlab.lib.utils import ImageReader
from geometry import mark
from palette import REPO, TOKENS, NAME, INK, TEAL, WHITE, BLACK, MIST, MUTED, BORDER, WORDMARK, DARK_TEXT

SOURCE=Path(__file__).resolve().parent
ROOT=SOURCE.parent
TEMP=ROOT/'.build'
FONT=SOURCE/'fonts/Sora-600.ttf'
REGULAR=SOURCE/'fonts/PlusJakartaSans-400.ttf'
SEMI=SOURCE/'fonts/PlusJakartaSans-600.ttf'
DATE='2026-11-10'
jobs=[]


def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()


def write(name,text):
    p=ROOT/name;p.parent.mkdir(parents=True,exist_ok=True);p.write_text(text,encoding='utf-8');return p


def svg(body,w=1000,h=1000,title=NAME):
    return f'<svg xmlns="http://www.w3.org/2000/svg" width="{w}" height="{h}" viewBox="0 0 {w} {h}" role="img" aria-label="{title}"><title>{title}</title>{body}</svg>'


def wordmark(color,size=160):
    font=TTFont(FONT);glyphs=font.getGlyphSet();cmap=font.getBestCmap();scale=size/font['head'].unitsPerEm;x=0;parts=[]
    for char in NAME:
        name=cmap[ord(char)];pen=SVGPathPen(glyphs);glyphs[name].draw(pen)
        parts.append(f'<path d="{pen.getCommands()}" transform="translate({x:.5f} {size}) scale({scale:.8f} {-scale:.8f})"/>')
        x+=font['hmtx'][name][0]*scale
    return f'<g fill="{color}">{"".join(parts)}</g>',x


def export(name,body,w=1000,h=1000,pixels=2048):
    p=write(f'svg/{name}.svg',svg(body,w,h,name));out=ROOT/f'png/{name}.png';out.parent.mkdir(exist_ok=True)
    jobs.append({'input':str(p),'output':str(out),'width':pixels})


def vectors():
    for label,color,base in [('primary',INK,TEAL),('ink',INK,INK),('white',WHITE,WHITE),('black',BLACK,BLACK),('teal',TEAL,TEAL)]:
        export(f'serverku-mark-{label}',mark(color,base))
    for label,color in [('ink',INK),('white',WHITE),('black',BLACK)]:
        export(f'serverku-mark-micro-{label}',mark(color,micro=True))
        letters,w=wordmark(WORDMARK if label in ('ink','primary') else DARK_TEXT if label=='white' else BLACK)
        export(f'serverku-wordmark-{label}',f'<g transform="translate(25 10)">{letters}</g>',int(w+50),220,2400)
    for label,color,base in [('primary',INK,TEAL),('ink',INK,INK),('white',WHITE,WHITE),('black',BLACK,BLACK)]:
        letters,w=wordmark(WORDMARK if label in ('ink','primary') else DARK_TEXT if label=='white' else BLACK)
        body=f'<g transform="translate(0 0) scale(.32)">{mark(color,base)}</g><g transform="translate(365 62)">{letters}</g>'
        export(f'serverku-horizontal-{label}',body,int(w+400),320,2400)
    for label,color,base in [('primary',INK,TEAL),('ink',INK,INK),('white',WHITE,WHITE)]:
        letters,w=wordmark(WORDMARK if label in ('ink','primary') else DARK_TEXT,155)
        body=f'<g transform="translate(150 0) scale(.7)">{mark(color,base)}</g><g transform="translate({(1000-w)/2:g} 680)">{letters}</g>'
        export(f'serverku-stacked-{label}',body,1000,920)
    for label,bg,fg in [('dark',INK,WHITE),('light',WHITE,INK)]:
        body=f'<rect width="1000" height="1000" rx="210" fill="{bg}"/><g transform="translate(125 100) scale(.75)">{mark(fg,micro=True)}</g>'
        export(f'serverku-app-icon-{label}',body)


def render():
    TEMP.mkdir(exist_ok=True);p=TEMP/'render-jobs.json';p.write_text(json.dumps(jobs))
    subprocess.run(['node',str(SOURCE/'render.mjs'),str(p)],check=True)


def icons():
    folder=ROOT/'icons';folder.mkdir(exist_ok=True)
    for label in ['dark','light']:
        im=Image.open(ROOT/f'png/serverku-app-icon-{label}.png')
        for size in [16,24,32,48,64,128,180,192,256,512,1024]:
            im.resize((size,size),Image.Resampling.LANCZOS).save(folder/f'serverku-app-{label}-{size}.png')
        im.save(folder/f'serverku-{label}.ico',sizes=[(16,16),(24,24),(32,32),(48,48),(64,64),(128,128),(256,256)])
    im=Image.open(ROOT/'png/serverku-app-icon-dark.png')
    im.resize((1024,1024),Image.Resampling.LANCZOS).save(folder/'serverku.icns',format='ICNS')
    iconset=folder/'serverku.iconset';iconset.mkdir(exist_ok=True)
    for base in [16,32,128,256,512]:
        for scale in [1,2]:
            im.resize((base*scale,base*scale),Image.Resampling.LANCZOS).save(iconset/f'icon_{base}x{base}{"@2x" if scale==2 else ""}.png')
    direct=[]
    for label in ['ink','white','black']:
        for size in [16,24,32,48,64]:direct.append({'input':str(ROOT/f'svg/serverku-mark-micro-{label}.svg'),'output':str(folder/f'serverku-symbol-{label}-{size}.png'),'width':size})
    p=TEMP/'icon-jobs.json';p.write_text(json.dumps(direct));subprocess.run(['node',str(SOURCE/'render.mjs'),str(p)],check=True)
    shutil.copy2(folder/'serverku-app-dark-32.png',folder/'favicon-32.png');shutil.copy2(folder/'serverku-app-dark-180.png',folder/'apple-touch-icon.png')
    write('icons/site.webmanifest',json.dumps({'name':NAME,'short_name':NAME,'icons':[{'src':f'serverku-app-dark-{s}.png','sizes':f'{s}x{s}','type':'image/png','purpose':'any'} for s in [192,512]],'theme_color':INK,'background_color':WHITE},indent=2)+'\n')


def paste(out,im,box,bg=WHITE):
    x,y,w,h=box;im=im.convert('RGBA');im.thumbnail((w,h),Image.Resampling.LANCZOS);layer=Image.new('RGBA',(w,h),bg)
    layer.alpha_composite(im,((w-im.width)//2,(h-im.height)//2));out.paste(layer.convert('RGB'),(x,y))


def sheets():
    regular=lambda s:ImageFont.truetype(str(REGULAR),s)
    semibold=lambda s:ImageFont.truetype(str(SEMI),s)
    def txt(d,xy,s,size=23,color=INK):d.text(xy,s,font=regular(size),fill=color)
    out=Image.new('RGB',(1800,1300),MIST);d=ImageDraw.Draw(out)
    d.text((70,50),NAME,font=ImageFont.truetype(str(FONT),67),fill=INK)
    txt(d,(72,149),'Compute when needed. Keep configured data between runs.',28)
    txt(d,(72,202),'APPROVED IDENTITY / SERVER + POWER / '+DATE,18,MUTED)
    d.rounded_rectangle((70,270,1110,910),radius=24,fill=WHITE)
    paste(out,Image.open(ROOT/'png/serverku-mark-primary.png'),(300,280,590,590))
    txt(d,(105,855),'ON-DEMAND COMPUTE / PERSISTENT STORAGE',20)
    d.rounded_rectangle((1140,270,1730,910),radius=24,fill=INK)
    paste(out,Image.open(ROOT/'png/serverku-mark-white.png'),(1200,320,470,470),INK)
    txt(d,(1180,849),'REVERSE / transparent power cutout',20,WHITE)
    d.rounded_rectangle((70,940,1730,1230),radius=24,fill=WHITE)
    paste(out,Image.open(ROOT/'png/serverku-horizontal-primary.png'),(100,960,1050,230))
    for i,col in enumerate([INK,TEAL,WHITE]):
        x=1310+i*125;d.rounded_rectangle((x,1010,x+95,1105),radius=15,fill=col,outline=BORDER);txt(d,(x,1130),col,16)
    out.save(ROOT/'serverku-overview.png')
    concept=Image.new('RGB',(1800,880),WHITE);d=ImageDraw.Draw(concept)
    panels=[('Primary / two-tone','serverku-mark-primary'),('One-color / same meaning','serverku-mark-ink'),('Micro / larger cutouts','serverku-mark-micro-ink')]
    for i,(label,name) in enumerate(panels):
        paste(concept,Image.open(ROOT/f'png/{name}.png'),(i*600+40,55,520,590));txt(d,(i*600+30,680),label,23)
    txt(d,(30,772),'Power control = disposable compute. Separate base = configured storage retained across down / up.',23,MUTED)
    concept.save(ROOT/'reference/design-rationale.png')
    small=Image.new('RGB',(1700,940),MIST);d=ImageDraw.Draw(small);txt(d,(35,25),'MICRO MARK / actual pixels above, nearest-neighbor enlargement below',22)
    for row,label in enumerate(['ink','white']):
        y=80+row*400;bg=WHITE if label=='ink' else INK;d.rounded_rectangle((30,y,1670,y+360),radius=20,fill=bg)
        for i,size in enumerate([16,24,32,48,64]):
            x=65+i*320;im=Image.open(ROOT/f'icons/serverku-symbol-{label}-{size}.png')
            paste(small,im,(x,y+25,size,size),bg);paste(small,im.resize((192,192),Image.Resampling.NEAREST),(x,y+105,192,192),bg)
            txt(d,(x,y+315),f'{size}px',19,INK if label=='ink' else WHITE)
    txt(d,(35,900),'At 16px use micro; inspect the real output. Platform integration is separate from this brand kit.',21)
    small.save(ROOT/'reference/small-size-check.png')
    detail=Image.new('RGB',(1800,930),WHITE);d=ImageDraw.Draw(detail);txt(d,(30,35),'CURVE REVIEW / original geometric source',30)
    im=Image.open(ROOT/'png/serverku-mark-primary.png')
    for i,(label,box) in enumerate([('Power arc and rounded cap',(770,430,1250,950)),('Chassis corner',(250,160,680,600)),('Storage detail',(440,1530,850,1750))]):
        paste(detail,im.crop(box),(i*600+30,150,540,500));txt(d,(i*600+30,710),label,24)
    txt(d,(30,835),'Exact arcs, transparent even-odd cutouts, no embedded raster imagery or masks. Approved palette.',23,MUTED)
    detail.save(ROOT/'reference/curve-detail-check.png')


def guide():
    for name,p in [('Body',REGULAR),('Semi',SEMI),('Logo',FONT)]:pdfmetrics.registerFont(RLFont(name,str(p)))
    c=canvas.Canvas(str(ROOT/'serverku-brand-guide.pdf'),pagesize=(842,595),invariant=1)
    c.setTitle('serverku - Brand guide');c.setAuthor(NAME)
    def rect(x,y,w,h,color,r=0):c.setFillColor(HexColor(color));c.roundRect(x,y,w,h,r,stroke=0,fill=1)
    def txt(x,y,s,size=12,color=INK,font='Body'):
        c.setFillColor(HexColor(color));c.setFont(font,size);c.drawString(x,y,s.replace('serverku', NAME))
    def para(x,y,s,width=350,size=12,color=MUTED,leading=19):
        line=''
        for word in s.split():
            trial=f'{line} {word}'.strip()
            if pdfmetrics.stringWidth(trial,'Body',size)>width:txt(x,y,line,size,color);y-=leading;line=word
            else:line=trial
        if line:txt(x,y,line,size,color);y-=leading
        return y
    def img(name,x,y,w,h):c.drawImage(ImageReader(str(ROOT/name)),x,y,width=w,height=h,mask='auto',preserveAspectRatio=True,anchor='c')
    def page(n,title,subtitle,kicker):
        rect(0,0,842,595,WHITE);txt(42,556,'SERVERKU / BRAND GUIDE',10,MUTED,'Semi');txt(680,556,DATE,10,MUTED)
        txt(42,510,title,30,INK,'Semi');txt(42,482,subtitle,11,MUTED);txt(42,25,kicker,9,MUTED);txt(760,25,f'{n:02d} / 07',9,MUTED)
    page(1,'Compute when you need it.','On-demand cloud VMs for Docker Compose projects.','IDENTITY / APPROVED 2026-11-10')
    txt(42,410,'serverku',48,INK,'Logo')
    para(44,358,'Create a runtime when needed. Keep configured project data on persistent storage between sessions.',330,16,INK,25)
    rect(440,130,360,330,MIST,22);img('png/serverku-mark-primary.png',470,146,300,300)
    para(44,231,'A server with a power control represents disposable compute. The separate base represents storage retained when the runtime is removed.',320,12)
    para(44,112,'Approved server + power artwork, palette, typography and placed-image rules. The headline remains suggested brand copy.',740,10)
    c.showPage()
    page(2,'One symbol. Two responsibilities.','Compute can come and go. Configured persistent storage can stay.','DESIGN RATIONALE')
    img('reference/design-rationale.png',30,150,782,300)
    para(42,126,'The primary mark uses two colors, but the same structure works in a single ink. The micro version widens the power cutout and replaces two storage details with one slot.',745,12)
    para(42,70,'This is original hand-authored vector artwork, not a reconstruction of a raster master. Serverku had no supplied logo image. Artwork and palette approved on 2026-11-10.',745,10)
    c.showPage()
    page(3,'A practical developer-tool identity.','Full mark, micro version and outlined lowercase lockups.','LOGO SYSTEM')
    for i,(name,label,desc) in enumerate([('serverku-mark-primary','Primary','Full server + power mark. Start at 48px.'),('serverku-mark-micro-ink','Micro','Wider cuts and one storage slot. Use at 16-32px.'),('serverku-mark-white','Reverse','White with transparent power and storage cuts.')]):
        x=42+i*260;rect(x,245,240,210,INK if i==2 else MIST,14);img(f'png/{name}.png',x+22,250,196,196)
        txt(x,220,label,17,INK,'Semi');para(x,195,desc,235,11)
    img('png/serverku-horizontal-primary.png',42,65,365,105)
    para(445,140,'The wordmark uses lowercase serverku to match the CLI. Approved lettering: Sora Semibold, delivered as vector outlines. Horizontal and stacked lockups are included.',350,11)
    c.showPage()
    page(4,'Grounded ink. Active teal.','Approved palette for graphical materials; terminal appearance stays user-controlled.','COLOR & TYPOGRAPHY')
    for i,(label,color,role) in enumerate([('Deep Teal',INK,'Compute / lettering / main ink'),('Active Teal',TEAL,'Storage base / digital accent'),('White',WHITE,'Background / reverse artwork')]):
        x=42+i*260;rect(x,310,240,140,MIST,12);rect(x+12,322,216,116,color,8);txt(x,282,label,17,INK,'Semi');txt(x,258,color,14,MUTED);txt(x,232,role,10,MUTED)
    txt(42,175,'serverku',33,INK,'Logo');para(42,130,'Sora Semibold 600: wordmark. Plus Jakarta Sans Regular/Semibold: guide and supporting copy. Font files and SIL OFL licenses are bundled.',350,11)
    txt(445,175,'Keep the CLI familiar.',17,INK,'Semi');para(445,145,'These are branding assets for docs, profiles and future product materials. They do not change terminal output, the user-selected terminal font or provider identities.',350,11)
    c.showPage()
    page(5,'Keep the control clear.','Placed-image rules; use separate, legibility-tested sizing for UI chrome.','USAGE & CLEAR SPACE')
    rect(42,170,350,280,MIST,14);img('png/serverku-mark-primary.png',122,220,190,190)
    # Visible normalized bounds are 140..860 x 100..900. Padding is 1/4 visible height.
    c.setStrokeColor(HexColor(TEAL));c.setDash(3,3);c.rect(110.6,201,212.8,228,stroke=1,fill=0);c.setDash()
    txt(68,184,'Clear space: 1/4 of visible mark height',10,MUTED)
    y=430
    for title,body in [('Choose contrast','Primary or deep teal on light surfaces. White on dark. Preserve true transparent cutouts.'),('Choose the right detail','Primary symbol: start at 48px; micro: 16-32px; horizontal lockup: start at 240px.'),('Keep the structure intact','Scale uniformly. Preserve the separation between compute and storage. Avoid gradients, shadows, provider logos or live-status recoloring.')]:
        txt(435,y,title,15,INK,'Semi');y=para(435,y-25,body,355,11)-20
    para(42,115,'Clear space is external to the visible artwork; the default export margins alone do not provide all of it. At 16px use the micro file and check the actual destination. The logo is an identity mark, not a live power control.',745,11)
    c.showPage()
    page(6,'Ready for docs and distribution.','Square tiles, transparent symbols and publishing formats.','ICONS & EXPORTS')
    img('png/serverku-app-icon-dark.png',42,250,180,180);img('png/serverku-app-icon-light.png',238,250,180,180)
    txt(450,420,'Publishing candidates',18,INK,'Semi');para(450,391,'PNG tiles from 16 to 1024px; ICO, ICNS, macOS iconset, favicon and Apple touch icon. These assets do not introduce a GUI, browser app or new platform support.',340,12)
    para(450,286,'Standalone micro marks are available in ink, white and black at 16, 24, 32, 48 and 64px. Use a high-contrast background and inspect the target rendering.',340,11)
    for row,label in enumerate(['ink','white']):
        y=146-row*70;bg=MIST if label=='ink' else INK;rect(42,y-12,758,65,bg,8)
        for i,size in enumerate([16,24,32,48,64]):
            x=65+i*145;img(f'icons/serverku-symbol-{label}-{size}.png',x,y,40,40);txt(x+52,y+14,f'{size}px',10,INK if label=='ink' else WHITE)
    c.showPage()
    page(7,'Be specific about the lifecycle.','Product facts and voice for consistent, accurate brand materials.','PRODUCT & HANDOFF')
    txt(42,432,'Who it is for',19,INK,'Semi');y=para(42,400,'Developers running side projects, previews, staging, demos and internal tools that do not need compute all day. Uses existing Docker Compose projects on GCP or DigitalOcean.',350,12)
    y=para(42,y-15,'up creates a VM and deploys. down removes the VM and keeps configured persistent storage. destroy deletes the tracked disk and local project configuration.',350,12)
    para(42,y-15,'Only data on configured persistent storage survives down. Retained storage and other provider charges can continue; do not promise zero cost or a fixed savings percentage.',350,12)
    txt(445,432,'How to talk about it',19,INK,'Semi');y=para(445,400,'Use direct, practical language: "Start your Compose project when needed" and "Keep configured data between sessions." Explain which resources remain and distinguish price estimates from bills.',350,12)
    para(445,y-16,'Avoid "serverless," "free when stopped," "your data can never be lost," or claims that down is simply a paused VM. Stay provider-neutral.',350,12)
    para(42,100,'Handoff: original SVGs, PNGs, icon candidates, brand-information.md, licensed fonts and portable source. Terminal appearance and cloud behavior are unchanged.',745,10)
    c.showPage();c.save()


# Local-only files never enter the manifest (shared with scripts/check_brand.py).
LOCAL_ONLY = {'.build', 'node_modules', '__pycache__', '.DS_Store', '.venv', 'venv'}
GENERATED_DIRS = ['svg', 'png', 'icons']
GENERATED_FILES = ['serverku-overview.png', 'serverku-brand-guide.pdf', 'brand-information.md', 'manifest.json',
                   'reference/repo-context.json', 'reference/design-rationale.png',
                   'reference/small-size-check.png', 'reference/curve-detail-check.png']


def clean():
    # Start from nothing generated, so an output the builder stops making cannot survive.
    for name in GENERATED_DIRS:
        shutil.rmtree(ROOT/name, ignore_errors=True)
    for name in GENERATED_FILES:
        (ROOT/name).unlink(missing_ok=True)


def file_list():
    return sorted(p for p in ROOT.rglob('*') if p.is_file() and p.name!='manifest.json' and p.suffix!='.zip' and not any(part in LOCAL_ONLY for part in p.relative_to(ROOT).parts))


def package():
    write('manifest.json',json.dumps({'brand':NAME,'date':DATE,'files':{str(p.relative_to(ROOT)):{'sha256':sha(p),'bytes':p.stat().st_size} for p in file_list()}},indent=2)+'\n')


def context():
    # Only build inputs are hashed. The guide's lifecycle copy was reviewed against the
    # listed code but does not read it, so code edits must not force a kit rebuild.
    write('reference/repo-context.json', json.dumps({
        'source_files': {'brand/tokens.json': {'sha256': sha(REPO/'brand/tokens.json')}},
        'lifecycle_copy_reviewed_against': ['cmd/serverku/lifecycle.go', 'internal/orchestrator/orchestrator.go'],
        'scope': 'Graphical brand kit and documentation; terminal output unchanged.'
    }, indent=2)+'\n')
    substitutions = {'name': NAME, **{key: value['value'] for key, value in TOKENS['colors'].items()}}
    write('brand-information.md', Template((SOURCE/'brand-information.template.md').read_text()).substitute(substitutions))


def main():
    clean();context();vectors();render();icons();sheets();guide();package();print(f'Built {len(jobs)} original Serverku SVG/PNG variants and seven-page PDF.')


if __name__=='__main__':main()
