import os
import sys, datetime as dt, statistics as st
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from lib import *

rows = load('T1')
grid = fill_gaps(to_grid(rows))
# сгруппировать по "суточному циклу" с началом в 03:00
byday = defaultdict(list)
for t,c in grid:
    anchor = t - dt.timedelta(hours=3)
    byday[anchor.date()].append((t,c))

print("=== T1: границы активности по дням (окно счётчика = 3ч, поэтому вход = C растёт) ===")
print("дата        дн  первый рост C   последний рост C   C=0 с      max")
first_rise=[]; last_rise=[]; zero_at=[]
names=['Пн','Вт','Ср','Чт','Пт','Сб','Вс']
for d in sorted(byday):
    seq = byday[d]
    fr = None; lr = None; z = None
    for i in range(1,len(seq)):
        if seq[i][1] > seq[i-1][1]:
            if fr is None: fr = seq[i][0]
            lr = seq[i][0]
    # первый момент после lr, когда C=0
    if lr:
        for t,c in seq:
            if t > lr and c == 0:
                z = t; break
    mx = max(c for _,c in seq)
    wd = names[d.weekday()]
    print(f"{d}  {wd}  {fr.strftime('%H:%M') if fr else '  -  '}          "
          f"{lr.strftime('%H:%M') if lr else '  -  '}            "
          f"{z.strftime('%H:%M') if z else '  -  '}     {mx:3d}")
    if fr: first_rise.append(fr.hour*60+fr.minute)
    if lr: last_rise.append(lr.hour*60+lr.minute)
    if z:  zero_at.append((z.hour*60+z.minute) % (24*60))

def hm(x): return f"{int(x)//60:02d}:{int(x)%60:02d}"
print(f"\nмедиана первого роста C:  {hm(st.median(first_rise))}   (≈ открытие)")
print(f"медиана последнего роста: {hm(st.median(last_rise))}   (≈ последний вход)")
print(f"медиана обнуления C:      {hm(st.median(zero_at))}")
print(f"\nпроверка: последний вход + 3ч = {hm((st.median(last_rise)+180)%(24*60))}  "
      f"vs фактическое обнуление {hm(st.median(zero_at))}")
