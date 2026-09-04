import os
import sys, datetime as dt, statistics as st
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from lib import *

rows = load('T1')
grid = fill_gaps(to_grid(rows))
print("сетка:", len(grid), "точек,", grid[0][0], "→", grid[-1][0])

# --- часы работы: когда счётчик стабильно 0 ---
byslot = defaultdict(list)
for t,c in grid:
    byslot[t.hour*60+t.minute].append(c)
print("\n=== доля нулей по времени суток (все дни августа, T1) ===")
zs = []
for s in sorted(byslot):
    v = byslot[s]; frac = sum(1 for x in v if x==0)/len(v)
    zs.append((s,frac))
for s,frac in zs:
    if s % 30: continue
    h,m = divmod(s,60)
    print(f"  {h:02d}:{m:02d}  нулей {frac*100:5.1f}%  ср={st.mean(byslot[s]):5.1f}")

# --- оценка N по критерию неотрицательности восстановленных входов ---
# C(t) = сумма входов в (t-N, t]  =>  a(t) = C(t) - C(t-1) + a(t-N)
def deconv(series, N_min):
    L = N_min // STEP           # число шагов в окне N
    a = [0.0]*len(series)
    neg = 0.0; negcnt = 0
    for i in range(1, len(series)):
        prev_a = a[i-L] if i-L >= 0 else 0.0
        val = series[i] - series[i-1] + prev_a
        if val < -1e-9:
            neg += -val; negcnt += 1
        a[i] = val
    return a, neg, negcnt

vals = [c for _,c in grid]
print("\n=== выбор окна N (критерий: восстановленные входы не должны быть отрицательными) ===")
print(" N(ч)   сумма отриц.  доля отриц.точек   сумма входов  вх/сут")
best=None
for Nm in [120,150,180,210,240,270,300]:
    a,neg,negcnt = deconv(vals, Nm)
    tot = sum(x for x in a if x>0)
    days = (grid[-1][0]-grid[0][0]).total_seconds()/86400
    frac = negcnt/len(a)
    print(f" {Nm/60:4.1f}   {neg:11.0f}   {frac*100:8.2f}%       {tot:10.0f}   {tot/days:6.0f}")
    if best is None or neg < best[1]: best=(Nm,neg)
print("\nминимум нарушений при N =", best[0]/60, "ч")
