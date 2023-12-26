all: gpsprom hanprom upsprom

.PHONY: gpsprom
gpsprom:
	mkdir -p bin
	make -C cmd/gpsprom

.PHONY: hanprom
hanprom:
	mkdir -p bin
	make -C cmd/hanprom

.PHONY: upsprom
upsprom:
	mkdir -p bin
	make -C cmd/upsprom
