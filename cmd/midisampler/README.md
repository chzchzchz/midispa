# midisampler

Virtual sampler midi device

## sine wave soundfont

Generate sine wave soundfont:
```c
double midi_to_hz(int midi_note) {
    static const double half_step = 1.0594630943592953;  
    static const double midi_c0 = 8.175798915643707;
    midi_note += 12;
    return midi_c0 * pow(half_step, midi_note);
}
```

```sh
for a in `./a.out`; do ffmpeg -f lavfi -i "sine=frequency=$a:duration=5" `printf "%04d" $a`.wav; done
```
