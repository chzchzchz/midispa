package yamaha

const (
	bulkVoiceCount       = 32
	bulkDataSize         = bulkVoiceCount * packedVoiceSize
	bulkMessageSize      = sysexHeaderSize + bulkDataSize + 2
	singleMessageSize    = sysexHeaderSize + singleVoiceDataSize + 2
	parameterMessageSize = 7
)

// SingleVoice is the 155-byte DX7 edit-buffer representation of a voice.
type SingleVoice struct {
	Channel int `range:"0..15"`
	Voice
}

// MarshalBinary encodes a complete 32-voice DX7 bulk-dump message.
func (b *BulkData) MarshalBinary() ([]byte, error) {
	return b.encode()
}

// UnmarshalBinary decodes a complete 32-voice DX7 bulk-dump message.
func (b *BulkData) UnmarshalBinary(data []byte) error {
	if len(data) != bulkMessageSize ||
		data[0] != 0xf0 ||
		data[1] != 0x43 ||
		data[2]&0xf0 != 0 ||
		data[3] != 0x09 ||
		data[4] != 0x20 ||
		data[5] != 0x00 ||
		data[len(data)-1] != 0xf7 {
		return errBadRange
	}
	payload := data[sysexHeaderSize : len(data)-2]
	if !valid7Bit(payload) || maskedChecksum(payload) != data[len(data)-2] {
		return errBadRange
	}

	var voices [bulkVoiceCount]Voice
	for i := range voices {
		start := i * packedVoiceSize
		if err := voices[i].UnmarshalBinary(payload[start : start+packedVoiceSize]); err != nil {
			return err
		}
	}
	*b = BulkData{Channel: int(data[2]), Voices: voices}
	return nil
}

// MarshalBinary encodes a Yamaha voice or function parameter-change message.
func (pc *ParameterChange) MarshalBinary() ([]byte, error) {
	return pc.encode()
}

// UnmarshalBinary decodes a Yamaha voice or function parameter-change message.
func (pc *ParameterChange) UnmarshalBinary(data []byte) error {
	if len(data) != parameterMessageSize ||
		data[0] != 0xf0 ||
		data[1] != 0x43 ||
		data[2]&0xf0 != 0x10 ||
		data[len(data)-1] != 0xf7 {
		return errBadRange
	}
	if !valid7Bit(data[3:6]) {
		return errBadRange
	}

	candidate := ParameterChange{
		Channel:   int(data[2] & 0x0f),
		Group:     int(data[3] >> 2),
		Parameter: int(data[3]&0x03)<<7 | int(data[4]),
		Data:      int(data[5]),
	}
	if err := candidate.check(); err != nil {
		return err
	}
	*pc = candidate
	return nil
}

// MarshalBinary encodes the packed 128-byte voice payload used in a bulk dump.
func (v *Voice) MarshalBinary() ([]byte, error) {
	return v.encode()
}

// UnmarshalBinary decodes the packed 128-byte voice payload used in a bulk dump.
func (v *Voice) UnmarshalBinary(data []byte) error {
	if len(data) != packedVoiceSize || !valid7Bit(data) {
		return errBadRange
	}

	var voice Voice
	for i := range voice.Osc {
		start := i * packedOperatorSize
		if err := voice.Osc[i].UnmarshalBinary(data[start : start+packedOperatorSize]); err != nil {
			return err
		}
	}
	for i := range voice.PitchEgRate {
		voice.PitchEgRate[i] = int(data[102+i])
	}
	for i := range voice.PitchEgLevel {
		voice.PitchEgLevel[i] = int(data[106+i])
	}
	voice.Algorithm = int(data[110] & 0x1f)
	voice.Feedback = int(data[111] & 0x07)
	voice.OscSync = int((data[111] >> 3) & 0x01)
	voice.LfoSpeed = int(data[112])
	voice.LfoDelay = int(data[113])
	voice.LfoPitchModDepth = int(data[114])
	voice.LfoAmpModDepth = int(data[115])
	voice.LfoSync = int(data[116] & 0x01)
	voice.LfoWaveform = int((data[116] >> 1) & 0x07)
	voice.PitchModSens = int((data[116] >> 4) & 0x07)
	voice.Transpose = int(data[117])
	copy(voice.VoiceName[:], data[118:128])
	if err := voice.check(); err != nil {
		return err
	}
	*v = voice
	return nil
}

// MarshalBinary encodes one packed 17-byte operator payload.
func (o *Osc) MarshalBinary() ([]byte, error) {
	return o.encode()
}

// Reserved high bits in packed DX7 records are ignored as don't-care bits.
func (o *Osc) UnmarshalBinary(data []byte) error {
	if len(data) != packedOperatorSize || !valid7Bit(data) {
		return errBadRange
	}

	var osc Osc
	for i := range osc.EgRate {
		osc.EgRate[i] = int(data[i])
	}
	for i := range osc.EgLevel {
		osc.EgLevel[i] = int(data[4+i])
	}
	osc.BrkPt = int(data[8])
	osc.LftDepth = int(data[9])
	osc.RhtDepth = int(data[10])
	osc.LftCurve = int((data[11] >> 2) & 0x03)
	osc.RhtCurve = int(data[11] & 0x03)
	osc.RateScale = int(data[12] & 0x07)
	osc.Detune = int((data[12] >> 3) & 0x0f)
	osc.ModSens = int(data[13] & 0x03)
	osc.VelSens = int((data[13] >> 2) & 0x07)
	osc.OutLevel = int(data[14])
	osc.Mode = int(data[15] & 0x01)
	osc.FreqCoarse = int((data[15] >> 1) & 0x1f)
	osc.FreqFine = int(data[16])
	if err := osc.check(); err != nil {
		return err
	}
	*o = osc
	return nil
}

// SingleVoice.MarshalBinary encodes the one-voice SysEx message.
func (sv *SingleVoice) MarshalBinary() ([]byte, error) {
	if err := checkTaggedFields(sv); err != nil {
		return nil, err
	}
	if err := sv.Voice.check(); err != nil {
		return nil, err
	}
	payload, err := sv.Voice.encodeUnpacked()
	if err != nil {
		return nil, err
	}
	ret := []byte{0xf0, 0x43, byte(sv.Channel), 0x00, 0x01, 0x1b}
	ret = append(ret, payload...)
	ret = append(ret, maskedChecksum(payload), 0xf7)
	return ret, nil
}

// UnmarshalBinary decodes a complete one-voice DX7 edit-buffer message.
func (sv *SingleVoice) UnmarshalBinary(data []byte) error {
	if len(data) != singleMessageSize ||
		data[0] != 0xf0 ||
		data[1] != 0x43 ||
		data[2]&0xf0 != 0 ||
		data[3] != 0x00 ||
		data[4] != 0x01 ||
		data[5] != 0x1b ||
		data[len(data)-1] != 0xf7 {
		return errBadRange
	}
	payload := data[sysexHeaderSize : len(data)-2]
	if !valid7Bit(payload) || maskedChecksum(payload) != data[len(data)-2] {
		return errBadRange
	}

	var voice Voice
	if err := voice.unmarshalUnpacked(payload); err != nil {
		return err
	}
	candidate := SingleVoice{Channel: int(data[2]), Voice: voice}
	if err := checkTaggedFields(&candidate); err != nil {
		return err
	}
	*sv = candidate
	return nil
}

func (v *Voice) encodeUnpacked() ([]byte, error) {
	if err := v.check(); err != nil {
		return nil, err
	}
	ret := make([]byte, 0, singleVoiceDataSize)
	for _, osc := range v.Osc {
		operator, err := osc.unpacked()
		if err != nil {
			return nil, err
		}
		ret = append(ret, operator...)
	}
	for _, value := range v.PitchEgRate {
		ret = append(ret, byte(value))
	}
	for _, value := range v.PitchEgLevel {
		ret = append(ret, byte(value))
	}
	ret = append(ret,
		byte(v.Algorithm),
		byte(v.Feedback),
		byte(v.OscSync),
		byte(v.LfoSpeed),
		byte(v.LfoDelay),
		byte(v.LfoPitchModDepth),
		byte(v.LfoAmpModDepth),
		byte(v.LfoSync),
		byte(v.LfoWaveform),
		byte(v.PitchModSens),
		byte(v.Transpose))
	ret = append(ret, v.VoiceName[:]...)
	return ret, nil
}

func (o *Osc) unpacked() ([]byte, error) {
	if err := o.check(); err != nil {
		return nil, err
	}
	ret := make([]byte, OscParams)
	for i, value := range o.EgRate {
		ret[i] = byte(value)
	}
	for i, value := range o.EgLevel {
		ret[4+i] = byte(value)
	}
	ret[8] = byte(o.BrkPt)
	ret[9] = byte(o.LftDepth)
	ret[10] = byte(o.RhtDepth)
	ret[11] = byte(o.LftCurve)
	ret[12] = byte(o.RhtCurve)
	ret[13] = byte(o.RateScale)
	ret[14] = byte(o.ModSens)
	ret[15] = byte(o.VelSens)
	ret[16] = byte(o.OutLevel)
	ret[17] = byte(o.Mode)
	ret[18] = byte(o.FreqCoarse)
	ret[19] = byte(o.FreqFine)
	ret[20] = byte(o.Detune)
	return ret, nil
}

func (v *Voice) unmarshalUnpacked(data []byte) error {
	if len(data) != singleVoiceDataSize || !valid7Bit(data) {
		return errBadRange
	}

	var voice Voice
	for i := range voice.Osc {
		start := i * OscParams
		var osc Osc
		for j := range osc.EgRate {
			osc.EgRate[j] = int(data[start+j])
		}
		for j := range osc.EgLevel {
			osc.EgLevel[j] = int(data[start+4+j])
		}
		osc.BrkPt = int(data[start+8])
		osc.LftDepth = int(data[start+9])
		osc.RhtDepth = int(data[start+10])
		osc.LftCurve = int(data[start+11])
		osc.RhtCurve = int(data[start+12])
		osc.RateScale = int(data[start+13])
		osc.ModSens = int(data[start+14])
		osc.VelSens = int(data[start+15])
		osc.OutLevel = int(data[start+16])
		osc.Mode = int(data[start+17])
		osc.FreqCoarse = int(data[start+18])
		osc.FreqFine = int(data[start+19])
		osc.Detune = int(data[start+20])
		if err := osc.check(); err != nil {
			return err
		}
		voice.Osc[i] = osc
	}
	for i := range voice.PitchEgRate {
		voice.PitchEgRate[i] = int(data[126+i])
	}
	for i := range voice.PitchEgLevel {
		voice.PitchEgLevel[i] = int(data[130+i])
	}
	voice.Algorithm = int(data[134])
	voice.Feedback = int(data[135])
	voice.OscSync = int(data[136])
	voice.LfoSpeed = int(data[137])
	voice.LfoDelay = int(data[138])
	voice.LfoPitchModDepth = int(data[139])
	voice.LfoAmpModDepth = int(data[140])
	voice.LfoSync = int(data[141])
	voice.LfoWaveform = int(data[142])
	voice.PitchModSens = int(data[143])
	voice.Transpose = int(data[144])
	copy(voice.VoiceName[:], data[145:155])
	if err := voice.check(); err != nil {
		return err
	}
	*v = voice
	return nil
}

// SysEx data bytes must be seven-bit values before packed fields are interpreted.
func valid7Bit(data []byte) bool {
	for _, value := range data {
		if value&0x80 != 0 {
			return false
		}
	}
	return true
}
