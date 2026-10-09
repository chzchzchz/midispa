all: clock.elf song_select.elf mmc_play_stop.elf volca_drum_velocity.elf
clean:
	rm *.elf

%.elf: %.c
	clang -O2 --target=bpf -c -o $@ $<