package mr

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/roylee0704/gron"
)

// 定义全局常量
const (
	TIMEOUT      = 10
	EMPTY_STATE  = 0
	MAP_STATE    = 1
	REDUCE_STATE = 2
)

// Master is a data structure describing the master machine in MapReduce
type Master struct {
	// Your definitions here.
	mu           sync.Mutex
	workCnts     int                 //worker的累计数量
	mapCnts      int                 //map task 的累计数量
	mappedCnts   int                 //map task完成的数量
	reduceCnts   int                 //reduce task 累计数量
	nReduce      int                 //reduce task的总数
	recoverMapID []int               // 回收后带分配的mapID
	waitFiles    []string            //还没有分配的split file
	reduceState  []int               //reduce 任务的状态,0未执行，1正在执行，2执行完成
	mappedFiles  map[string]bool     //保存已经map完的文件信息，防止重复map
	workerList   map[int]*WorkerInfo //保存所有worker信息
}

// WorkerInfo is a data structure describing the state of worker machine in MapReduce
type WorkerInfo struct {
	workerID   int   //工作进程唯一标志符
	startTime  int64 //上一次收到心跳信息的timestamp
	workStatus int   //0: 未工作，1：map任务，2：reduce任务
	mapID      int
	workInfo   string // 0: "", 1: map任务的文件名，2：reduce [0, nReduce]
}

// Your code here -- RPC handlers for the worker to call.

// Example is an example RPC handler.
//
// the RPC argument and reply types are defined in rpc.go.
//
func (m *Master) Example(args *ExampleArgs, reply *ExampleReply) error {
	reply.Y = args.X + 1
	return nil
}

func (m *Master) getAWorkerID() int {
	ret := m.workCnts
	m.workCnts++
	return ret
}

func (m *Master) getAMapID() int {

	var ret int

	if len(m.recoverMapID) > 0 {
		ret = m.recoverMapID[0]
		m.recoverMapID = m.recoverMapID[1:]
	} else {
		ret = m.mapCnts
		m.mapCnts++
	}

	return ret
}

// Register is a RPC method
func (m *Master) Register(args *EmptyArgs, reply *RegisterReply) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// 有剩余的Map task
	if len(m.waitFiles) > 0 {
		reply.MasterState = 1
	} else if m.reduceCnts < m.nReduce {
		reply.MasterState = 2
	} else {
		reply.MasterState = 0
	}
	reply.WorkerID = m.getAWorkerID()
	// 初始化工作进程的有关信息
	workerInfo := WorkerInfo{}
	workerInfo.workerID = reply.WorkerID
	workerInfo.workStatus = EMPTY_STATE
	workerInfo.workInfo = ""

	m.workerList[reply.WorkerID] = &workerInfo

	return nil
}

//NewMapTask is a RPC method
func (m *Master) NewMapTask(args *NewMapTaskArgs, reply *NewMapTaskReply) error {
	// return "", nil
	// 如果有文件还没有进行map， 分配一个map task
	m.mu.Lock()
	defer m.mu.Unlock()
	file := ""
	id := -1
	if l := len(m.waitFiles); l > 0 {
		file = m.waitFiles[0]

		id = m.getAMapID()
		if l == 1 {
			m.waitFiles = m.waitFiles[:0]
		} else {
			m.waitFiles = m.waitFiles[1:]
		}

		reply.Filename = file
		reply.Err = nil
		reply.Id = id
		reply.NReduce = m.nReduce

		// 更新worker状态
		m.workerList[args.WorkerID].workStatus = MAP_STATE
		m.workerList[args.WorkerID].workInfo = file
		m.workerList[args.WorkerID].startTime = time.Now().Unix()
		m.workerList[args.WorkerID].mapID = id
		// debug
		fmt.Println("[INFO]分配一个split，文件名： ", file)

	} else {
		reply.Err = errors.New("[INFO]No extra map task")
	}
	return nil
}

// NewReduceTask is a RPC method
func (m *Master) NewReduceTask(args *NewReduceTaskArgs, reply *NewReduceTaskReply) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	var target = -1
	for i := 0; i < m.nReduce; i++ {
		if m.reduceState[i] == 0 {
			target = i
			break
		}
	}
	reply.Target = target
	reply.MapCnts = m.mapCnts
	if target == -1 {
		reply.Err = errors.New("[INFO]There is no extra reduce task")
	} else {
		m.workerList[args.WorkerID].workStatus = REDUCE_STATE
		m.workerList[args.WorkerID].workInfo = strconv.Itoa(target)
		m.workerList[args.WorkerID].startTime = time.Now().Unix()
		m.reduceState[target] = 1
		m.reduceCnts++
		fmt.Println("[INFO]分配一个reduce task，编号： ", target)
	}
	return nil
}

func (m *Master) zeroWork(workerID int) {
	m.workerList[workerID].workStatus = EMPTY_STATE
	m.workerList[workerID].workInfo = ""
}

//FinishMapTask is a RPC method
func (m *Master) FinishMapTask(args *FinishArgs, reply *FinishReply) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	workerID := args.WorkerID
	mapID := args.ID
	timeNow := time.Now().Unix()
	m.workerList[workerID].workStatus = EMPTY_STATE
	if timeNow-m.workerList[workerID].startTime > 10 || m.mappedFiles[args.Filename] {
		fmt.Printf("[Master]%s已经map\n", args.Filename)
		return errors.New("已经map")
	}
	// 将workerID 执行map任务得到的临时文件重新命名
	for i := 0; i < m.nReduce; i++ {
		src := "mr-out-" + strconv.Itoa(mapID) + "-" + strconv.Itoa(i) + "-" + strconv.Itoa(workerID) + ".tmp"
		dst := "mr-out-" + strconv.Itoa(mapID) + "-" + strconv.Itoa(i)
		os.Rename(src, dst)
	}

	m.mappedCnts++
	m.mappedFiles[args.Filename] = true
	m.zeroWork(workerID)
	return nil
}

//FinishReduceTask is a RPC method
func (m *Master) FinishReduceTask(args *FinishArgs, reply *FinishReply) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	workerID := args.WorkerID
	reduceID := args.ID
	timeNow := time.Now().Unix()
	m.workerList[workerID].workStatus = EMPTY_STATE
	// 防止重复reduce
	if timeNow-m.workerList[workerID].startTime > 10 || m.reduceState[reduceID] == 2 {
		fmt.Printf("[Master]%d已经map\n", reduceID)
		return errors.New("已经reduce")
	}
	// 重命名
	src := "mr-out-" + strconv.Itoa(reduceID) + "-" + strconv.Itoa(workerID) + ".tmp"
	dst := "mr-out-" + strconv.Itoa(reduceID)

	os.Rename(src, dst)

	m.reduceState[reduceID] = 2
	m.zeroWork(workerID)
	return nil
}

//DeleteWorker is a RPC method
func (m *Master) DeleteWorker(args *FinishArgs, reply *FinishReply) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.workerList, args.WorkerID)
	return nil
}

// GetState is a RPC method
func (m *Master) GetState(args *EmptyArgs, reply *MasterStateReply) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.checkExit() {
		reply.MasterState = 3 // 是否mapreduce已完成
	} else if m.reduceCnts < m.nReduce && m.mappedCnts == m.mapCnts && len(m.waitFiles) == 0 {
		reply.MasterState = 2 // 是否所有map task 均已完成
	} else if len(m.waitFiles) > 0 {
		reply.MasterState = 1 // 是否有剩余的map task
	} else {
		reply.MasterState = 0
	}
	reply.Err = nil
	return nil
}

//
// start a thread that listens for RPCs from worker.go
//
func (m *Master) server() {
	rpc.Register(m)
	rpc.HandleHTTP()
	//l, e := net.Listen("tcp", ":1234")
	sockname := masterSock()
	os.Remove(sockname)
	l, e := net.Listen("unix", sockname)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go http.Serve(l, nil)
}

func (m *Master) checkExit() bool {
	for _, x := range m.reduceState {
		if x != 2 {
			return false
		}
	}
	return true
}

// check every worker
func (m *Master) checkWorkers() {
	m.mu.Lock()
	defer m.mu.Unlock()
	timeNow := time.Now().Unix()
	for _, worker := range m.workerList {
		if worker.workStatus == MAP_STATE && timeNow-worker.startTime > 10 {
			// recover the map task
			m.recoverMapID = append(m.recoverMapID, worker.mapID)
			m.waitFiles = append(m.waitFiles, worker.workInfo)
		} else if worker.workStatus == REDUCE_STATE && timeNow-worker.startTime > 10 {
			tar, _ := strconv.Atoi(worker.workInfo)
			m.reduceState[tar] = 0
			m.reduceCnts--
		}
	}
}

// Done is a RPC method
// main/mrmaster.go calls Done() periodically to find out
// if the entire job has finished.
//
func (m *Master) Done() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	// 所有的 worker退出，同时 mapreduce任务已经完成
	if len(m.workerList) == 0 && m.checkExit() {
		// 删除所有临时输出文件
		exec.Command("bash", "-c", "rm *.tmp")
		return true
	}
	return false
}

// MakeMaster is Exported function used by mrmaster
// create a Master.
// main/mrmaster.go calls this function.
// nReduce is the number of reduce tasks to use.
//
func MakeMaster(files []string, nReduce int) *Master {
	m := Master{}

	// Your code here.
	m.mapCnts = 0
	m.mappedCnts = 0
	m.waitFiles = files
	m.nReduce = nReduce
	m.reduceState = make([]int, nReduce)
	m.workerList = make(map[int]*WorkerInfo)
	m.mappedFiles = make(map[string]bool)

	fmt.Println(m.waitFiles)

	// 创建周期任务
	c := gron.New()
	c.AddFunc(gron.Every(5*time.Second), func() {
		m.checkWorkers()
	})
	c.Start()

	m.server()
	return &m
}
