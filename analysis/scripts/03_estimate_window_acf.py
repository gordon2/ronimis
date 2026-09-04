import os
import sys, datetime as dt, statistics as st, math
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from lib import *

# Если C(t) = сумма входов в окне L шагов, то dC(t) = a(t) - a(t-L).
# Тогда cov(dC(t), dC(t+L)) = -var(a) -> автокорреляция dC имеет ОСТРЫЙ МИНИМУМ на лаге L.
def acf_min(loc):
    rows = load(loc)
    grid = fill_gaps(to_grid(rows))
    v = [c for _,c in grid]
    d = [v[i]-v[i-1] for i in range(1,len(v))]
    m = st.mean(d)
    dc = [x-m for x in d]
    var = sum(x*x for x in dc)
    res = []
    for lag in range(1, 200):   # до 400 минут
        s = sum(dc[i]*dc[i+lag] for i in range(len(dc)-lag))
        res.append((lag, s/var))
    return res

print("=== автокорреляция ΔC: ищем минимум (= окно счётчика N) ===\n")
for loc in ['T1','Mustika','Suur-Paala','Hipodroom']:
    res = acf_min(loc)
    lag, val = min(res, key=lambda x: x[1])
    print(f"{loc:12s} минимум на лаге {lag:3d} шагов = {lag*STEP:3d} мин = {lag*STEP/60:.2f} ч   (r={val:+.3f})")
    # покажем окрестность минимума
    near = [(l*STEP/60, r) for l,r in res if abs(l-lag) <= 30 and l % 5 == 0]
    print("   профиль:", "  ".join(f"{h:.1f}ч:{r:+.2f}" for h,r in near))
    print()

print("=== детально T1: ACF по лагам, кратным 15 мин ===")
res = acf_min('T1')
for lag, r in res:
    if lag*STEP % 15: continue
    bar = '#'*int(max(0,-r)*100)
    print(f"  {lag*STEP:4d} мин ({lag*STEP/60:4.2f} ч)  r={r:+.4f}  {bar}")
