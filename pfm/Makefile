# pfm — one entry point for building, gating, installing, and keeping the
# fleet's own processes honest.
#
# This file does NOT reimplement the repo gates. `.claude/scripts/dev.sh` owns
# `test`, `iso`, `verify`, and friends for every project; duplicating them here
# would create a second definition of "green" that drifts from the first. The
# targets below delegate to it, and add only what it does not own: the host
# mirror install, and the stale-process problem that install creates.
#
# THE STALE-PROCESS PROBLEM, stated once because every target below exists
# because of it: replacing ~/.local/bin/pfm does not touch a process already
# running the old image. Linux keeps the deleted inode alive for it, so a
# long-lived `pfm mcp serve` keeps serving yesterday's tool descriptions and a
# days-old `pfm ls` picker keeps writing yesterday's decisions over today's
# fixes. Both fail SILENTLY — the binary on disk is new, the behaviour is old,
# and nothing in between says a word. `make install` therefore restarts the
# daemon every time and reports every other stale process it can see.

SHELL := /usr/bin/env bash
# Real exit codes: a failing command in a pipe fails the target, instead of the
# last command in the pipe deciding the verdict.
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help

REPO    := $(abspath $(dir $(abspath $(lastword $(MAKEFILE_LIST))))..)
DEV     := $(REPO)/.claude/scripts/dev.sh
BIN     := $(HOME)/.local/bin/pfm
LOCAL   := pfm.dev
UNIT    := pfm-mcp.service
# Stamped into every build below so `pfm --version` names the release it came
# from instead of the bare "dev" a plain `go build` leaves behind (issue #14
# F6). $(REPO)/VERSION, not a bare "VERSION" — this file runs from pfm/.
VERSION := $(shell cat $(REPO)/VERSION 2>/dev/null)
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

.PHONY: help build fmt fmt-check vet test iso gate install host-install rollback \
        mcp-restart mcp-status stale kill-stale sweep-stale clean

help: ## List every target
	@echo "pfm targets — run as: make <target>"
	@echo
	@grep -hE '^[a-z-]+:.*?## ' $(MAKEFILE_LIST) \
	  | sort \
	  | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'
	@echo
	@echo "Gates delegate to $(DEV) — this file never redefines them."

# ---------------------------------------------------------------- build ----

build: ## Compile to ./pfm.dev (a local artifact, never the shipped path)
	go build $(LDFLAGS) -o $(LOCAL) ./cmd/pfm

fmt: ## Rewrite formatting in place
	gofmt -w .

fmt-check: ## Fail if anything is unformatted, naming the files
	@unformatted="$$(gofmt -l . | grep -v '^$$' || true)"; \
	if [ -n "$$unformatted" ]; then \
	  echo "UNFORMATTED:"; echo "$$unformatted"; exit 1; \
	fi; \
	echo "gofmt: clean"

vet: ## go vet the whole module
	go vet ./...

# ----------------------------------------------------------------- gate ----
# Both delegate. dev.sh is the single definition of this project's gate.

test: ## Host test suite (delegates to dev.sh)
	bash $(DEV) test pfm

iso: ## Containerised test suite (delegates to dev.sh iso)
	bash $(DEV) iso test pfm

gate: fmt-check vet test iso ## Everything a change must pass before it merges
	@echo "gate: all passed"

# -------------------------------------------------------------- install ----

install: ## Swap in a new build atomically, restart the daemon, then sweep stale processes — any failure rolls back and sweeps nothing
	@$(MAKE) --no-print-directory host-install
	@if ! $(MAKE) --no-print-directory mcp-restart; then \
	  echo "install: FAILED at the MCP restart — rolling back to $(BIN).prev; no process was touched" >&2; \
	  $(MAKE) --no-print-directory rollback; \
	  exit 1; \
	fi
	@$(MAKE) --no-print-directory sweep-stale
	@echo
	@echo "install: complete — binary swapped, MCP daemon restarted, stale processes swept."
	@# Order is the safety: nothing is killed until the new binary is in place
	@# AND the daemon is serving it. A failed build or smoke test changes
	@# nothing; a failed restart restores the previous binary. Only a fully
	@# successful install sweeps, so a failure never leaves the host with the
	@# old processes gone and no working replacement.

host-install: ## Build + smoke-test a new pfm beside ~/.local/bin/pfm, then swap it in atomically (previous kept as pfm.prev)
	@mkdir -p "$$(dirname "$(BIN)")"; \
	tmp="$$(mktemp "$(BIN).new.XXXXXX")"; \
	trap 'rm -f "$$tmp"' EXIT; \
	go build $(LDFLAGS) -o "$$tmp" ./cmd/pfm || { echo "host-install: build FAILED — $(BIN) untouched" >&2; exit 1; }; \
	reported="$$("$$tmp" --version 2>&1)" || { echo "host-install: smoke test FAILED ($$reported) — $(BIN) untouched" >&2; exit 1; }; \
	case "$$reported" in *"$(VERSION)"*) ;; \
	  *) echo "host-install: smoke test reported '$$reported', not $(VERSION) — $(BIN) untouched" >&2; exit 1;; esac; \
	kept=""; if [ -x "$(BIN)" ]; then cp -f "$(BIN)" "$(BIN).prev" && kept="; previous kept at $(BIN).prev"; fi; \
	mv -f "$$tmp" "$(BIN)"; \
	echo "installed: $(BIN) ($$reported$$kept)"
	@# Never write onto the running path directly: an executing binary gives
	@# ETXTBSY. The build lands BESIDE the target — same directory, same
	@# filesystem — so the final mv is a rename(2): atomic, never a
	@# copy-over-the-running-image. Processes already running keep their own
	@# image, which is exactly why the sweep below has to exist.

rollback: ## Restore ~/.local/bin/pfm from pfm.prev atomically and restart the daemon on it
	@[ -x "$(BIN).prev" ] || { echo "rollback: no $(BIN).prev to restore — $(BIN) left as is" >&2; exit 1; }
	@tmp="$$(mktemp "$(BIN).rollback.XXXXXX")"; \
	cp -f "$(BIN).prev" "$$tmp" && chmod 755 "$$tmp" && mv -f "$$tmp" "$(BIN)" \
	  || { rm -f "$$tmp"; echo "rollback: FAILED to restore $(BIN) from $(BIN).prev" >&2; exit 1; }; \
	echo "rollback: restored $(BIN) from $(BIN).prev"
	@$(MAKE) --no-print-directory mcp-restart \
	  || echo "rollback: the daemon did not come back on the restored binary either — systemctl --user status $(UNIT)" >&2

# ---------------------------------------------------------------- mcp ------

mcp-restart: ## Restart the MCP daemon so it serves the binary now on disk
	@if command -v systemctl >/dev/null 2>&1 && \
	    systemctl --user cat $(UNIT) >/dev/null 2>&1; then \
	  systemctl --user restart $(UNIT); \
	  sleep 1; \
	  state="$$(systemctl --user is-active $(UNIT) || true)"; \
	  if [ "$$state" != "active" ]; then \
	    echo "MCP-RESTART-FAILED: $(UNIT) is '$$state' after restart — the fleet has NO chat MCP server" >&2; \
	    exit 1; \
	  fi; \
	  pid="$$(systemctl --user show -p MainPID --value $(UNIT))"; \
	  echo "mcp: $(UNIT) restarted, pid $$pid"; \
	elif pgrep -f 'pfm mcp serve' >/dev/null 2>&1; then \
	  echo "mcp: no $(UNIT) unit; killing unsupervised 'pfm mcp serve' so the next client starts a fresh one"; \
	  pkill -f 'pfm mcp serve'; \
	else \
	  echo "mcp: no daemon running and no $(UNIT) unit — nothing to restart"; \
	fi
	@# Restarting unconditionally is deliberate. Deciding whether the daemon is
	@# stale needs process introspection that not every platform offers, and a
	@# restart is cheap, supervised, and idempotent — so the guarantee does not
	@# depend on a probe that might not be able to run.

mcp-status: ## Report the daemon and whether it is running a replaced binary
	@if command -v systemctl >/dev/null 2>&1 && \
	    systemctl --user cat $(UNIT) >/dev/null 2>&1; then \
	  echo "unit:  $$(systemctl --user is-active $(UNIT))"; \
	  pid="$$(systemctl --user show -p MainPID --value $(UNIT))"; \
	  echo "pid:   $$pid"; \
	  if [ -r "/proc/$$pid/exe" ]; then \
	    echo "image: $$(readlink /proc/$$pid/exe)"; \
	  else \
	    echo "image: UNKNOWN — cannot read /proc/$$pid/exe on this platform"; \
	  fi; \
	else \
	  echo "unit:  not installed"; \
	  pgrep -af 'pfm mcp serve' || echo "no 'pfm mcp serve' process running"; \
	fi

# --------------------------------------------------------------- stale -----

stale: ## List pfm processes still running a replaced binary
	@if [ ! -d /proc ]; then \
	  echo "STALE-UNKNOWN: this platform has no /proc, so stale pfm processes could NOT be enumerated." >&2; \
	  echo "  This is not a clean result. The MCP daemon was restarted regardless, so it is fresh;" >&2; \
	  echo "  any other long-lived pfm process (a 'pfm ls' picker) may still hold the old binary." >&2; \
	  exit 0; \
	fi; \
	found=0; \
	for pid in $$(pgrep -x pfm 2>/dev/null || true); do \
	  link="$$(readlink /proc/$$pid/exe 2>/dev/null || true)"; \
	  case "$$link" in \
	    *"(deleted)"*) \
	      found=$$((found + 1)); \
	      cmd="$$(tr '\0' ' ' < /proc/$$pid/cmdline 2>/dev/null | cut -c1-70)"; \
	      echo "STALE pid=$$pid  $$cmd";; \
	  esac; \
	done; \
	if [ "$$found" -eq 0 ]; then \
	  echo "stale: none — every running pfm process uses the binary now on disk"; \
	else \
	  echo "stale: $$found process(es) above are running a REPLACED binary."; \
	  echo "       They will keep behaving like the old build and can write stale"; \
	  echo "       state over new fixes. Close them, or run: make kill-stale"; \
	fi
	@# "none" and "could not look" are printed differently on purpose. A sweep
	@# that reports nothing found when it never managed to look is the exact
	@# shape this repo treats as a defect.

kill-stale: ## Kill pfm processes running a replaced binary (interactive pickers included)
	@if [ ! -d /proc ]; then \
	  echo "REFUSING: no /proc, so stale processes cannot be identified — killing pfm processes blind would take down live ones" >&2; \
	  exit 1; \
	fi; \
	killed=0; \
	for pid in $$(pgrep -x pfm 2>/dev/null || true); do \
	  link="$$(readlink /proc/$$pid/exe 2>/dev/null || true)"; \
	  case "$$link" in \
	    *"(deleted)"*) \
	      cmd="$$(tr '\0' ' ' < /proc/$$pid/cmdline 2>/dev/null | cut -c1-70)"; \
	      echo "killing pid=$$pid  $$cmd"; \
	      kill "$$pid" 2>/dev/null || echo "  could not signal pid=$$pid (already gone, or not ours)"; \
	      killed=$$((killed + 1));; \
	  esac; \
	done; \
	echo "kill-stale: signalled $$killed process(es)"
	@# The manual, unverified form. `install` runs sweep-stale instead — and
	@# only after the new binary is in place and the daemon serves it.

sweep-stale: ## Kill every pfm process still running a replaced binary, then prove none is left
	@if [ ! -d /proc ]; then \
	  echo "STALE-UNKNOWN: this platform has no /proc, so stale pfm processes could NOT be enumerated or swept." >&2; \
	  echo "  This is not a clean result: the daemon was restarted, but a long-lived 'pfm' picker may still hold the old binary — close it by hand." >&2; \
	  exit 0; \
	fi; \
	stale_pids() { \
	  for pid in $$(pgrep -x pfm 2>/dev/null || true); do \
	    case "$$(readlink /proc/$$pid/exe 2>/dev/null || true)" in *"(deleted)"*) echo "$$pid";; esac; \
	  done; \
	}; \
	pids="$$(stale_pids)"; \
	if [ -z "$$pids" ]; then echo "sweep: none — every running pfm process uses the binary now on disk"; exit 0; fi; \
	mcp=0; \
	for pid in $$pids; do \
	  cmd="$$(tr '\0' ' ' < /proc/$$pid/cmdline 2>/dev/null | cut -c1-70)"; \
	  case "$$cmd" in *" mcp "*) mcp=1;; esac; \
	  echo "sweep: TERM pid=$$pid  $$cmd"; \
	  kill "$$pid" 2>/dev/null || true; \
	done; \
	for _ in 1 2 3 4 5 6; do [ -z "$$(stale_pids)" ] && break; sleep 0.5; done; \
	for pid in $$(stale_pids); do echo "sweep: KILL pid=$$pid (ignored TERM for 3s)"; kill -9 "$$pid" 2>/dev/null || true; done; \
	sleep 0.2; \
	left="$$(stale_pids)"; \
	if [ -n "$$left" ]; then echo "SWEEP-INCOMPLETE: still running a replaced binary: $$left" >&2; exit 1; fi; \
	[ "$$mcp" -eq 0 ] || echo "sweep: a session whose chat MCP server was swept reconnects it with /mcp"; \
	echo "sweep: done — no pfm process runs a replaced binary"
	@# Readback, not trust: the sweep re-scans after signalling and fails
	@# loudly on a survivor. It matches only images the kernel marks
	@# "(deleted)", so the binary just installed is never touched.

clean: ## Remove local build artifacts
	rm -f $(LOCAL)
	@echo "clean: removed $(LOCAL)"
