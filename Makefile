# webrtc-apm — build / test / native dep management.
#
# `make deps` builds the two native libraries this project links against
# (webrtc-audio-processing 1.x and rnnoise) from source and installs them
# into $(PREFIX), default /usr/local. Run once per machine, and again
# when WEBRTC_APM_TAG bumps.

WEBRTC_APM_REPO := https://gitlab.freedesktop.org/pulseaudio/webrtc-audio-processing.git
WEBRTC_APM_TAG  := v1.3
RNNOISE_REPO    := https://github.com/xiph/rnnoise.git

DEPS_DIR := $(CURDIR)/.deps
PREFIX   ?= /usr/local

.PHONY: all build test test-framer vet tidy clean
.PHONY: deps deps-build deps-install deps-clean
.PHONY: deps-check-toolchain

all: build

build:
	go build ./...

test:
	go test ./... -race -count=1

# Pure-Go tests only — runs without the native deps installed.
test-framer:
	go test ./framer/... -race -count=1

vet:
	go vet ./...

tidy:
	go mod tidy

clean:
	go clean -cache -testcache

# -----------------------------------------------------------------
# Native deps: source-build webrtc-audio-processing-1 and rnnoise.
# -----------------------------------------------------------------

deps: deps-build deps-install
	@echo
	@echo "Native deps installed under $(PREFIX). You may need to run:"
	@echo "    sudo ldconfig"
	@echo "so the linker picks them up."

deps-check-toolchain:
	@command -v meson       >/dev/null || { echo "missing 'meson'"; exit 1; }
	@command -v ninja       >/dev/null || { echo "missing 'ninja'"; exit 1; }
	@command -v pkg-config  >/dev/null || { echo "missing 'pkg-config'"; exit 1; }
	@command -v autoreconf  >/dev/null || { echo "missing 'autoreconf' (autoconf)"; exit 1; }
	@command -v libtoolize  >/dev/null || command -v glibtoolize >/dev/null || { echo "missing 'libtool'"; exit 1; }
	@command -v git         >/dev/null || { echo "missing 'git'"; exit 1; }
	@pkg-config --exists absl_base 2>/dev/null || { echo "missing abseil-cpp dev headers (libabsl-dev)"; exit 1; }

deps-build: deps-check-toolchain | $(DEPS_DIR)
	@if [ ! -d $(DEPS_DIR)/webrtc-audio-processing ]; then \
	    git clone --depth 1 --branch $(WEBRTC_APM_TAG) \
	        $(WEBRTC_APM_REPO) $(DEPS_DIR)/webrtc-audio-processing; \
	fi
	cd $(DEPS_DIR)/webrtc-audio-processing && \
	    meson setup --prefix=$(PREFIX) --reconfigure build && \
	    meson compile -C build
	@if [ ! -d $(DEPS_DIR)/rnnoise ]; then \
	    git clone --depth 1 $(RNNOISE_REPO) $(DEPS_DIR)/rnnoise; \
	fi
	cd $(DEPS_DIR)/rnnoise && \
	    ./autogen.sh && \
	    ./configure --prefix=$(PREFIX) && \
	    $(MAKE)

# Installs into $(PREFIX). When PREFIX=/usr/local (default) this needs
# sudo; override PREFIX=$$HOME/.local for a user-local install and set
# PKG_CONFIG_PATH + LD_LIBRARY_PATH accordingly.
deps-install:
	@if [ "$$(stat -c %u $(PREFIX) 2>/dev/null || stat -f %u $(PREFIX) 2>/dev/null)" = "0" ] && [ "$$(id -u)" != "0" ]; then \
	    SUDO=sudo; \
	else \
	    SUDO=; \
	fi; \
	$$SUDO meson install -C $(DEPS_DIR)/webrtc-audio-processing/build; \
	cd $(DEPS_DIR)/rnnoise && $$SUDO $(MAKE) install

deps-clean:
	rm -rf $(DEPS_DIR)

$(DEPS_DIR):
	mkdir -p $(DEPS_DIR)
