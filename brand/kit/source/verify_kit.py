"""Structural, transparency, platform-format and portable-handoff checks."""
import json
import struct
import xml.etree.ElementTree as ET
import subprocess
import sys
from PIL import Image
from pypdf import PdfReader
from build_kit import ROOT, sha
from palette import REPO, TOKENS


def main():
    assert not list(ROOT.rglob('*.go')),'Go context snapshots must use .go.txt to stay outside package discovery'
    provenance=json.loads((ROOT/'reference/historical-provenance.json').read_text())
    for name,item in provenance['source_files'].items():
        if name.startswith('source/fonts/'):assert sha(ROOT/name)==item['sha256'],name
    ns='{http://www.w3.org/2000/svg}';svgs=sorted((ROOT/'svg').glob('*.svg'));assert len(svgs)==20
    for p in svgs:
        doc=ET.parse(p).getroot();assert doc.tag==ns+'svg' and doc.find(ns+'title') is not None,p
        assert list(doc.iter(ns+'path')),p
        assert all(e.tag not in {ns+'image',ns+'text',ns+'mask',ns+'script',ns+'style'} for e in doc.iter()),p
        for e in doc.iter():assert not any(k.endswith('href') for k in e.attrib),p
    pngs=sorted((ROOT/'png').glob('*.png'));assert len(pngs)==20
    for p in pngs:
        with Image.open(p) as im:
            im.load();assert im.mode=='RGBA',p
            a=im.getchannel('A');assert a.getextrema()==(0,255),p
            if 'app-icon' not in p.name:
                l,t,r,b=a.getbbox();assert l>0 and t>0 and r<im.width and b<im.height,p
            assert im.width==(2400 if 'wordmark' in p.name or 'horizontal' in p.name else 2048),p
    def components(token):
        value=TOKENS['colors'][token]['value'].lstrip('#')
        return tuple(int(value[i:i+2],16) for i in (0,2,4))
    for label,token in [('ink','ink'),('white','white'),('black','black'),('teal','accent')]:
        expected=components(token)
        im=Image.open(ROOT/f'png/serverku-mark-{label}.png')
        assert {p[:3] for p in im.get_flattened_data() if p[3]==255}=={expected},label
        assert im.getpixel((1024,500))[3]==0,'Power cutout is not transparent'
    primary=Image.open(ROOT/'png/serverku-mark-primary.png');micro=Image.open(ROOT/'png/serverku-mark-micro-ink.png')
    assert {p[:3] for p in primary.get_flattened_data() if p[3]==255}=={components('ink'),components('accent')}
    for x,y in [(500,540),(300,805)]:
        pt=(round(x*2.048),round(y*2.048))
        assert primary.getpixel(pt)[3]==255 and micro.getpixel(pt)[3]==0,pt
    # Separation between compute and storage must remain transparent.
    assert primary.getpixel((1024,round(680*2.048)))[3]==0
    icon_pngs=sorted((ROOT/'icons').rglob('*.png'));assert len(icon_pngs)==49
    for p in icon_pngs:
        with Image.open(p) as im:
            im.load();assert im.mode=='RGBA' and im.width==im.height,p
            if p.name.startswith('serverku-symbol-') or p.name.startswith('serverku-app-'):
                assert im.width==int(p.stem.split('-')[-1]),p
    for label in ['dark','light']:
        raw=(ROOT/f'icons/serverku-{label}.ico').read_bytes();reserved,kind,n=struct.unpack_from('<HHH',raw)
        assert reserved==0 and kind==1;sizes=set()
        for i in range(n):
            w,h,_,_,planes,bits,length,offset=struct.unpack_from('<BBBBHHII',raw,6+16*i)
            assert length>0 and offset+length<=len(raw);sizes.add((w or 256,h or 256))
        assert sizes=={(s,s) for s in [16,24,32,48,64,128,256]}
    with Image.open(ROOT/'icons/serverku.icns') as im:im.load();assert im.size==(1024,1024)
    pdf=ROOT/'serverku-brand-guide.pdf';reader=PdfReader(pdf);assert len(reader.pages)==7
    for page in reader.pages:assert len(page.extract_text())>150 and float(page.mediabox.width)==842
    review=json.loads((ROOT/'reference/pdf-review.json').read_text())
    assert review['sha256']==sha(pdf) and review['pages_reviewed']==list(range(1,8)),'PDF review is stale'
    subprocess.run([sys.executable,str(REPO/'scripts/check_brand.py')],check=True)
    print('Kit verification passed: vectors, PNGs, icons, fonts and all seven PDF pages.')


if __name__=='__main__':main()
