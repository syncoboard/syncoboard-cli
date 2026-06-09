package ui

import (
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/gen2brain/malgo"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v4"
	"gopkg.in/hraban/opus.v2"
)

type AudioPlayer struct {
	ctx    *malgo.AllocatedContext
	device *malgo.Device

	buffers      map[string][]int16
	buffersMutex sync.Mutex

	isStarted bool
}

func NewAudioPlayer() (*AudioPlayer, error) {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, func(message string) {
		// Suppress ALSA logs or log them to file
	})
	if err != nil {
		return nil, err
	}
	return &AudioPlayer{
		ctx:     ctx,
		buffers: make(map[string][]int16),
	}, nil
}

func (a *AudioPlayer) startDevice() error {
	if a.isStarted {
		return nil
	}

	deviceConfig := malgo.DefaultDeviceConfig(malgo.Playback)
	deviceConfig.Playback.Format = malgo.FormatS16
	deviceConfig.Playback.Channels = 2
	deviceConfig.SampleRate = 48000
	deviceConfig.Alsa.NoMMap = 1

	onSendFrames := func(pOutputSample, pInputSamples []byte, framecount uint32) {
		a.buffersMutex.Lock()
		defer a.buffersMutex.Unlock()

		samplesNeeded := int(framecount) * 2 // 2 channels

		for i := 0; i < samplesNeeded; i++ {
			var mixedSample int32 = 0
			activeStreams := 0

			for trackID, buf := range a.buffers {
				if len(buf) > 0 {
					mixedSample += int32(buf[0])
					a.buffers[trackID] = buf[1:]
					activeStreams++
				}
			}

			// Simple mixing (clipping logic)
			if mixedSample > 32767 {
				mixedSample = 32767
			} else if mixedSample < -32768 {
				mixedSample = -32768
			}

			binary.LittleEndian.PutUint16(pOutputSample[i*2:], uint16(mixedSample))
		}
	}

	callbacks := malgo.DeviceCallbacks{
		Data: onSendFrames,
	}

	device, err := malgo.InitDevice(a.ctx.Context, deviceConfig, callbacks)
	if err != nil {
		return err
	}
	a.device = device
	a.isStarted = true

	return device.Start()
}

func (a *AudioPlayer) PlayTrack(track *webrtc.TrackRemote) error {
	err := a.startDevice()
	if err != nil {
		return err
	}

	// Setup Opus decoder for this specific track
	decoder, err := opus.NewDecoder(48000, 2)
	if err != nil {
		return fmt.Errorf("failed to create opus decoder: %w", err)
	}

	trackID := track.ID()

	a.buffersMutex.Lock()
	a.buffers[trackID] = make([]int16, 0)
	a.buffersMutex.Unlock()

	go func() {
		depacketizer := &codecs.OpusPacket{}
		pcmBuffer := make([]int16, 1920*2) // Max frame size for 48kHz is 120ms (5760 samples per channel). 1920 is 40ms.

		for {
			rtp, _, err := track.ReadRTP()
			if err != nil {
				break
			}

			payload, err := depacketizer.Unmarshal(rtp.Payload)
			if err != nil {
				continue
			}

			n, err := decoder.Decode(payload, pcmBuffer)
			if err != nil {
				continue
			}

			totalSamples := n * 2

			a.buffersMutex.Lock()
			a.buffers[trackID] = append(a.buffers[trackID], pcmBuffer[:totalSamples]...)
			if len(a.buffers[trackID]) > 48000*2 {
				a.buffers[trackID] = a.buffers[trackID][len(a.buffers[trackID])-48000*2:]
			}
			a.buffersMutex.Unlock()
		}

		a.buffersMutex.Lock()
		delete(a.buffers, trackID)
		a.buffersMutex.Unlock()
	}()

	return nil
}

func (a *AudioPlayer) Close() {
	if a.device != nil {
		a.device.Uninit()
	}
	if a.ctx != nil {
		a.ctx.Uninit()
		a.ctx.Free()
	}
}
