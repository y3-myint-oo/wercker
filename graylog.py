import csv
import sys

reader = csv.DictReader(open(sys.argv[1]))

listed = list()

for line in reader:
    if "[x] docker: " in line['message']:
        continue
    listed.append((line['timestamp'], line['message']))

for line in sorted(listed):
    print line
