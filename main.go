package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"math/rand"
	"miningPoolCli/config"
	"miningPoolCli/utils/api"
	"miningPoolCli/utils/boc"
	"miningPoolCli/utils/files"
	"miningPoolCli/utils/gpuwrk"
	"miningPoolCli/utils/helpers"
	"miningPoolCli/utils/initp"
	"miningPoolCli/utils/logreport"
	"miningPoolCli/utils/mlog"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type gpuGoroutine struct {
	gpuData gpuwrk.GPUstruct
	// startTimestamp int64

	procStderr bytes.Buffer
}

var gpuGoroutines []gpuGoroutine
var globalTasks []api.Task

func startTask(i int, task api.Task) {
	// gpuGoroutines[i].startTimestamp = time.Now().Unix()

	mineResultFilename := "mined_" + strconv.Itoa(task.Id) + ".boc"
	pathToBoc := config.MinerGetter.MinerDirectory + "/" + mineResultFilename

	cmd := exec.Command(
		"./"+config.MinerGetter.MinerDirectory+"/pow-miner-opencl", "-vv",
		"-g"+strconv.Itoa(gpuGoroutines[i].gpuData.GpuId),
		"-p"+strconv.Itoa(gpuGoroutines[i].gpuData.PlatformId),
		"-F"+strconv.Itoa(config.StaticBeforeMinerSettings.BoostFactor),
		"-t"+strconv.Itoa(config.StaticBeforeMinerSettings.TimeoutT),
		config.StaticBeforeMinerSettings.PoolAddress,
		helpers.ConvertHexData(task.Seed),
		helpers.ConvertHexData(task.Complexity),
		config.StaticBeforeMinerSettings.Iterations,
		task.Giver,
		pathToBoc,
	)

	cmd.Stderr = &gpuGoroutines[i].procStderr

	unblockFunc := make(chan struct{}, 1)

	var killedByNotActual bool
	var done bool

	cmd.Start()

	go func() {
		cmd.Wait()
		done = true

		if helpers.StringInSlice(mineResultFilename, files.GetDir(config.MinerGetter.MinerDirectory)) {
			// found
			if !killedByNotActual {
				bocFileInHex, _ := boc.ReadBocFileToHex(pathToBoc)

				bocServerResp := api.SendHexBocToServer(bocFileInHex, task.Seed, strconv.Itoa(task.Id))
				if bocServerResp.Data == "Found" && bocServerResp.Status == "ok" {
					logreport.ShareFound(gpuGoroutines[i].gpuData.Model, gpuGoroutines[i].gpuData.GpuId, task.Id)
				} else {
					logreport.ShareServerError(task, bocServerResp, gpuGoroutines[i].gpuData.GpuId)
				}
			}
			files.RemovePath(pathToBoc)
		}
		enableTask(i)
		unblockFunc <- struct{}{}
	}()

	go func() {
		for !done {
			if checkTaskAlreadyFound(task.Id) {
				killedByNotActual = true
				if err := cmd.Process.Kill(); err != nil {
					mlog.LogError(err.Error())
				}
				break
			}
			time.Sleep(1 * time.Second)
		}
	}()

	<-unblockFunc
}

func checkTaskAlreadyFound(checkId int) bool {
	for _, task := range globalTasks {
		if task.Id == checkId {
			return false
		}
	}

	return true
}

func syncTasks(firstSync *chan struct{}) {
	var firstSyncIsOk bool
	for {
		globalTasks = api.GetTasks().Tasks

		if len(globalTasks) > 0 && !firstSyncIsOk {
			*firstSync <- struct{}{}
			firstSyncIsOk = true
		}

		time.Sleep(1 * time.Second)
	}
}

func enableTask(gpuGoIndex int) {
	go startTask(gpuGoIndex, globalTasks[rand.Intn(len(globalTasks))])
}

type mStats struct {
	HashRate int `json:"hash_rate"`
	Uptime   int `json:"uptime"`
}

func calcHashrate(gpus []gpuGoroutine) {
	var totalHashRate int

	for _, v := range gpus {
		hsArr := config.MRgxKit.FindHashRate.FindAllString(v.procStderr.String(), -1)
		if len(hsArr) < 2 {
			return
		}
		lastHs := hsArr[len(hsArr)-1]
		hS := config.MRgxKit.FindDecimal.FindAllString(lastHs, -1)

		sep := strings.Split(hS[0], ".")
		if len(sep) != 2 {
			return
		}

		perHashRate, err := strconv.Atoi(sep[0])
		if err != nil {
			return
		}

		totalHashRate += perHashRate
	}

	if config.UpdateStatsFile {
		var genStats mStats = mStats{
			HashRate: totalHashRate,
			Uptime:   int(time.Now().Unix()) - config.StartProgramTimestamp,
		}

		file, err := json.Marshal(genStats)
		if err != nil {
			mlog.LogFatalStackError(err)
		}
		_ = ioutil.WriteFile("stats.json", file, 0644)
	}

	mlog.LogInfo("Total hashrate: ~" + strconv.Itoa(totalHashRate) + " Mh")
}

func main() {
	rand.Seed(time.Now().Unix())
	gpus := initp.InitProgram()

	gpuGoroutines = make([]gpuGoroutine, len(gpus))

	firstSync := make(chan struct{})
	go syncTasks(&firstSync)
	<-firstSync

	for gpuGoIndex := range gpuGoroutines {
		gpuGoroutines[gpuGoIndex].gpuData = gpus[gpuGoIndex]
		enableTask(gpuGoIndex)
	}

	for {
		calcHashrate(gpuGoroutines)
		time.Sleep(60 * time.Second)
	}
}
