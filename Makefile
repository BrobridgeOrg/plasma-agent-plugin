PYTHON ?= python3

.PHONY: check test build release

check: test

test:
	$(PYTHON) -m unittest discover -s scripts -p 'test_*.py'

build: release

release:
	$(PYTHON) scripts/package_plugin.py
