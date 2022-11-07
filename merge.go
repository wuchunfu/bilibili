package bilibili

import (
	"context"
	"errors"
	codec "github.com/yapingcat/gomedia/go-codec"
	flv "github.com/yapingcat/gomedia/go-flv"
	mp4 "github.com/yapingcat/gomedia/go-mp4"
	"io"
	"os"
	"strconv"
)

type MergeTsFileListToSingleMp4_Req struct {
	FlvFileList    []string
	OutputMp4      string
	Ctx            context.Context
	FlvTotalLength int64
}

func MergeFlvFileListToSingleMp4(req MergeTsFileListToSingleMp4_Req) (err error) {
	mp4file, err := os.OpenFile(req.OutputMp4, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0666)
	if err != nil {
		return err
	}
	defer mp4file.Close()

	muxer, err := mp4.CreateMp4Muxer(mp4file)
	if err != nil {
		return err
	}
	vtid := muxer.AddVideoTrack(mp4.MP4_CODEC_H264)
	atid := muxer.AddAudioTrack(mp4.MP4_CODEC_AAC)

	var OnFrameErr error
	var curLength int64

	for _, flvFile := range req.FlvFileList {
		demuxer := flv.CreateFlvReader()
		demuxer.OnFrame = func(cid codec.CodecID, frame []byte, pts uint32, dts uint32) {
			if OnFrameErr != nil {
				return
			}
			if cid == codec.CODECID_AUDIO_AAC {
				OnFrameErr = muxer.Write(atid, frame, uint64(pts), uint64(dts))
			} else if cid == codec.CODECID_VIDEO_H264 {
				OnFrameErr = muxer.Write(vtid, frame, uint64(pts), uint64(dts))
			} else {
				OnFrameErr = errors.New("unknown cid " + strconv.Itoa(int(cid)))
				return
			}
		}
		err = readFileToCb(flvFile, func(buf []byte) error {
			err0 := demuxer.Input(buf)
			if err0 != nil {
				return err0
			}
			select {
			case <-req.Ctx.Done():
				return req.Ctx.Err()
			default:
			}
			curLength += int64(len(buf))
			FnUpdateProgress(float64(curLength) / float64(req.FlvTotalLength))
			return nil
		})
		if err != nil {
			return err
		}
		if OnFrameErr != nil {
			return OnFrameErr
		}
	}

	err = muxer.WriteTrailer()
	if err != nil {
		return err
	}
	err = mp4file.Sync()
	if err != nil {
		return err
	}
	FnUpdateProgress(0)
	return nil
}

func readFileToCb(fileName string, cb func(buf []byte) error) error {
	f, err := os.Open(fileName)
	if err != nil {
		return err
	}
	defer f.Close()

	var buf = make([]byte, 2*1024*1024) // 2MB
	for {
		var n int
		n, err = f.Read(buf)
		var isEof = false
		if err == io.EOF {
			isEof = true
		} else if err != nil {
			return err
		}
		err = cb(buf[:n])
		if err != nil {
			return err
		}
		if isEof {
			break
		}
	}
	return nil
}
