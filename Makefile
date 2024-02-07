all: gpsprom hanprom upsprom ocppprom

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

.PHONY: ocppprom
ocppprom:
	mkdir -p bin
	make -C cmd/ocppprom
