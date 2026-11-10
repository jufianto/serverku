"""Original Serverku vector proposal: switchable compute, persistent base.

Analytic circular arcs and rounded rectangles; no tracing, masks or rasters.
"""
from math import cos, sin, radians

from palette import INK, TEAL, WHITE


def rounded_rect(x,y,w,h,r):
    return (f'M{x+r:g} {y:g} H{x+w-r:g} A{r:g} {r:g} 0 0 1 {x+w:g} {y+r:g} '
            f'V{y+h-r:g} A{r:g} {r:g} 0 0 1 {x+w-r:g} {y+h:g} '
            f'H{x+r:g} A{r:g} {r:g} 0 0 1 {x:g} {y+h-r:g} '
            f'V{y+r:g} A{r:g} {r:g} 0 0 1 {x+r:g} {y:g} Z')


def ring(inner=101,outer=145):
    cap=(outer-inner)/2
    def pt(radius,angle):
        return f'{500+radius*cos(radians(angle)):.6f} {365+radius*sin(radians(angle)):.6f}'
    return (f'M{pt(outer,-50)} A{outer} {outer} 0 1 1 {pt(outer,-130)} '
            f'A{cap:g} {cap:g} 0 0 1 {pt(inner,-130)} '
            f'A{inner} {inner} 0 1 0 {pt(inner,-50)} '
            f'A{cap:g} {cap:g} 0 0 1 {pt(outer,-50)} Z')


def mark(color=INK,base=None,micro=False):
    stem=64 if micro else 44
    upper=rounded_rect(140,100,720,550,96)
    power=(ring(122,190) if micro else ring())+' '+rounded_rect(500-stem/2,150 if micro else 180,stem,210 if micro else 175,stem/2)
    lower=rounded_rect(140,710,720,190,64)
    if micro:
        slots=rounded_rect(275,779,450,52,26)
    else:
        slots=rounded_rect(245,781,48,48,24)+' '+rounded_rect(365,785,370,40,20)
    return (f'<path d="{upper} {power}" fill="{color}" fill-rule="evenodd"/>'
            f'<path d="{lower} {slots}" fill="{base or color}" fill-rule="evenodd"/>')
