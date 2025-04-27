all: clock.elf song_select.elf
clean:
	rm *.elf

clock.elf: clock.c
	clang --target=bpf -c -o $@ $<

song_select.elf: song_select.c
	clang -O2 --target=bpf -c -o $@ $<