"""Prints "x y", the centre of skid's first node in a uiautomator dump on stdin
whose text, content-desc or class is LABEL; exits 1 when none is on screen.

usage: centre.py LABEL < dump.xml
"""
import re
import sys
import xml.etree.ElementTree as ElementTree

label = sys.argv[1]
for node in ElementTree.parse(sys.stdin).iter('node'):
    if (node.get('package') == 'dev.niels.skidbladnir'
            and label in (node.get('text'), node.get('content-desc'), node.get('class'))):
        left, top, right, bottom = map(int, re.findall(r'-?\d+', node.get('bounds')))
        if right > left and bottom > top:
            print((left + right) // 2, (top + bottom) // 2)
            sys.exit(0)
sys.exit(f'no node of skid on screen is {label!r}')
