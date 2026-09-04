import os
import sys, datetime as dt, statistics as st
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from lib import *

WIN = 180          # окно счётчика, мин (установлено по ACF)
G   = 10           # сетка анализа, мин
L   = WIN // G     # шагов в окне = 18
NS  = 24*60 // G   # 144 слота в сутках
OFF = 3*60 // G    # старт суток в 03:00 (гарантированный ноль) = слот 18

def sat_profile(loc, weekday=5, pat="gym-stats-202608*.csv"):
    """Профиль дня недели на сетке G мин, начиная с 03:00. Медиана по дням."""
    rows = load(loc, pat)
    acc = defaultdict(list)
    for ts,c in rows:
        # слот относительно 03:00 того "суточного цикла", к которому относится точка
        anchor = ts - dt.timedelta(hours=3)
        if anchor.weekday()!=weekday: continue
        idx = (anchor.hour*60 + anchor.minute)//G
        acc[idx].append(c)
    prof=[]; ndays=[]
    for i in range(NS):
        v = acc.get(i,[])
        prof.append(st.median(v) if v else 0.0)
        ndays.append(len(v))
    return prof, ndays

def slot_label(i):
    """Метка слота: i=0 -> 03:00."""
    m = (i*G + 3*60) % (24*60)
    return f"{m//60:02d}:{m%60:02d}"

def deconvolve(C, L, lam=3.0, iters=60000, lr=0.02):
    """NNLS: min ||A a - C||^2 + lam*||a''||^2, a>=0. A = скользящая сумма окна L."""
    n = len(C)
    a = [max(0.0, (C[i]-C[i-1]) if i else 0.0) for i in range(n)]
    # тёплый старт: равномерно размазать
    a = [max(0.0, C[i]/L) for i in range(n)]
    for it in range(iters):
        # residual r = A a - C ; (A a)[i] = sum_{k=0..L-1} a[i-k]
        Aa=[0.0]*n; s=0.0
        for i in range(n):
            s += a[i]
            if i-L >= 0: s -= a[i-L]
            Aa[i]=s
        r=[Aa[i]-C[i] for i in range(n)]
        # grad_fit = A^T r ; (A^T r)[j] = sum_{i=j..j+L-1} r[i]
        g=[0.0]*n; s=0.0
        for j in range(n-1,-1,-1):
            s += r[j]
            if j+L < n: s -= r[j+L]
            g[j]=s
        # регуляризация гладкости: d2 = a[i-1]-2a[i]+a[i+1]
        for i in range(n):
            d2 = 0.0
            if 0<i<n-1: d2 = a[i-1]-2*a[i]+a[i+1]
            g[i] += -lam*d2*2
        mx = max(abs(x) for x in g) or 1.0
        step = lr/ (1.0 + it/8000)
        for i in range(n):
            a[i] = max(0.0, a[i] - step*g[i]/mx*max(1.0,mx)*0.02)
        if it%20000==0:
            rms = (sum(x*x for x in r)/n)**0.5
            print(f"    iter {it:6d}  rms={rms:.4f}")
    Aa=[0.0]*n; s=0.0
    for i in range(n):
        s += a[i]
        if i-L>=0: s-=a[i-L]
        Aa[i]=s
    rms=(sum((Aa[i]-C[i])**2 for i in range(n))/n)**0.5
    return a, Aa, rms

C, nd = sat_profile('T1')
print("=== T1 суббота: профиль (медиана 5 суббот), сетка 10 мин, отсчёт с 03:00 ===")
print("дней в каждом слоте:", min(nd), "–", max(nd))
print("\nдеконволюция (окно 180 мин):")
a, Aa, rms = deconvolve(C, L)
print(f"  итог rms реконструкции = {rms:.4f}  (при медианном C ~ {st.median([x for x in C if x>0]):.1f})")

import json
json.dump({'C':C,'a':a,'Aa':Aa,'G':G,'L':L,'OFF':OFF},
          open(os.path.join(os.path.dirname(os.path.abspath(__file__)),'..','results','t1_sat.json'),'w'))

print("\nвремя   C(набл)  A·a(рекон)   a=входы/10мин   входы/час")
for i in range(NS):
    if C[i]==0 and a[i]<0.05 and (i*G+180)%(24*60) not in range(0,0): pass
    lbl = slot_label(i)
    if i%3: continue
    print(f"{lbl}   {C[i]:6.1f}   {Aa[i]:8.2f}     {a[i]:8.2f}      {a[i]*6:6.1f}")
