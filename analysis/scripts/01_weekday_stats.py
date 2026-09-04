import csv, glob, os, datetime as dt, statistics as st
from collections import defaultdict

RT = os.path.expanduser("~/Library/Application Support/ronimis")
rows = []
for f in sorted(glob.glob(os.path.join(RT, "gym-stats-202608*.csv"))):
    with open(f, newline='') as fh:
        r = csv.DictReader(fh)
        for rec in r:
            if rec.get('status') != 'success': continue
            if rec.get('location_name') != 'T1': continue
            try:
                ts = dt.datetime.strptime(rec['timestamp'], '%Y-%m-%d %H:%M:%S')
                cnt = int(rec['user_count'])
            except Exception:
                continue
            rows.append((ts, cnt))

rows.sort()
print("T1 записей за август:", len(rows))
print("диапазон:", rows[0][0], "→", rows[-1][0])

bydate = defaultdict(list)
for ts, c in rows:
    bydate[ts.date()].append((ts, c))

print("\nдней:", len(bydate))
sats = [d for d in sorted(bydate) if d.weekday() == 5]
print("субботы августа 2026:", [str(d) for d in sats])
for d in sats:
    v = [c for _, c in bydate[d]]
    print(f"  {d}  точек={len(v):4d}  max={max(v):3d}  ненулевых={sum(1 for x in v if x>0):4d}")

# по дням недели: средний максимум
print("\n=== по дню недели (T1, август) ===")
names = ['Пн','Вт','Ср','Чт','Пт','Сб','Вс']
for wd in range(7):
    ds = [d for d in sorted(bydate) if d.weekday() == wd]
    mx = [max(c for _, c in bydate[d]) for d in ds]
    tot = [sum(c for _, c in bydate[d]) for d in ds]
    print(f"  {names[wd]}  дней={len(ds)}  ср.пик={st.mean(mx):5.1f}  медиана пика={st.median(mx):5.1f}  ср.сумма={st.mean(tot):8.0f}")
