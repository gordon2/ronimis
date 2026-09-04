import csv, glob, os, datetime as dt, statistics as st
from collections import defaultdict

RT = os.path.expanduser("~/Library/Application Support/ronimis")
def load(loc, pat="gym-stats-202608*.csv"):
    rows=[]
    for f in sorted(glob.glob(os.path.join(RT,pat))):
        with open(f,newline='') as fh:
            for rec in csv.DictReader(fh):
                if rec.get('status')!='success' or rec.get('location_name')!=loc: continue
                try:
                    ts=dt.datetime.strptime(rec['timestamp'],'%Y-%m-%d %H:%M:%S'); c=int(rec['user_count'])
                except Exception: continue
                rows.append((ts,c))
    rows.sort(); return rows

rows = load('T1')
# профиль суббот: слот = 10 минут
slots = defaultdict(list)
for ts,c in rows:
    if ts.weekday()!=5: continue
    slot = ts.hour*60 + (ts.minute//10)*10
    slots[slot].append(c)

print("=== T1, СУББОТА: профиль (медиана по 5 субботам августа) ===")
print("время   мед  сред  мин  макс   бар")
keys = sorted(slots)
for s in keys:
    v = slots[s]
    med = st.median(v); mean = st.mean(v)
    h, m = divmod(s, 60)
    bar = '#' * int(round(med))
    print(f"{h:02d}:{m:02d}  {med:5.1f} {mean:5.1f} {min(v):4d} {max(v):4d}   {bar}")
