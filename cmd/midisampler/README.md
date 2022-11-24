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

## Live sampling

Connect any jack source to the sampler's jack sink to record samples.

1. Hold record button to collect sample.
2. Hold play to replay sample.
3. Seek forward to chop beginning of sample.
4. Seek back to chop end of sample.
5. Press loop and the keyboard key to assign. Press stop to cancel.
6. Press stop to reset to original sample.
