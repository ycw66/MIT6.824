package mr

import (
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"io/ioutil"
	"log"
	"net/rpc"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

//
// Map functions return a slice of KeyValue.
//
type KeyValue struct {
	Key   string
	Value string
}

//
// use ihash(key) % NReduce to choose the reduce
// task number for each KeyValue emitted by Map.
//
func ihash(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() & 0x7fffffff)
}

//Worker is the exported function used by mrworker
// main/mrworker.go calls this function.
//
func Worker(mapf func(string, string) []KeyValue,
	reducef func(string, []string) string) {

	RegiserInfo := RegisterWork()

	if RegiserInfo.Err != nil {
		log.Fatal(RegiserInfo.Err)
	}

	workerID := RegiserInfo.WorkerID
	masterState := RegiserInfo.MasterState

	for true {
		if masterState == 1 {
			// 请求map-task
			fmt.Println("[INFO]请求Map task...")
			taskInfo := RequestMapTask(workerID)
			// 正常返回任务
			if taskInfo.Err == nil {
				// 成功执行
				fmt.Println("[INFO]执行Map task")
				if err := ExecuteMapTask(workerID, taskInfo, mapf); err == nil {
					// 更改workInfo
					if CallFinishMapTask(workerID, taskInfo.Id, taskInfo.Filename) == nil {
						fmt.Printf("[INFO]成功执行Map task, mapFile:%s, workerId:%d\n", taskInfo.Filename, workerID)
					}
				}
			}
		} else if masterState == 2 {
			// 请求reduce-task
			fmt.Println("[INFO]请求Reduce task")
			taskInfo := RequestReduceTask(workerID)
			if taskInfo.Err == nil {
				// 执行Reduce
				if err := ExecuteReduceTask(workerID, taskInfo, reducef); err == nil {
					if CallFinishReduceTask(workerID, taskInfo.Target) == nil {
						fmt.Println("[INFO]成功执行Reduce task")
					}
				}
			}
		} else if masterState == 3 {
			//退出
			//TODO: 结束其他的线程
			CallDeleteWorker(workerID)
			return
		} else {
			fmt.Println("[INFO]没有多余的任务，等一等...")
		}
		fmt.Println("[INFO]本轮任务完成")
		time.Sleep(time.Second)
		fmt.Println("[INFO]寻找新的任务")

		masterState = GetMasterState()
	}
}

//
// example function to show how to make an RPC call to the master.
//
// the RPC argument and reply types are defined in rpc.go.
//
func CallExample() {

	// declare an argument structure.
	args := ExampleArgs{}

	// fill in the argument(s).
	args.X = 99

	// declare a reply structure.
	reply := ExampleReply{}

	// send the RPC request, wait for the reply.
	call("Master.Example", &args, &reply)

	// reply.Y should be 100.
	fmt.Printf("reply.Y %v\n", reply.Y)
}

// 注册
func RegisterWork() RegisterReply {
	reply := RegisterReply{}
	args := ExampleArgs{}
	call("Master.Register", &args, &reply)
	if reply.Err != nil {
		log.Fatal(reply)
	}
	fmt.Printf("[INFO]worker成功注册， workerID:%d\n", reply.WorkerID)
	return reply
}

func CallFinishMapTask(workerID int, mapID int, filename string) error {
	reply := FinishReply{}
	args := FinishArgs{}
	args.WorkerID = workerID
	args.ID = mapID
	args.Filename = filename
	call("Master.FinishMapTask", &args, &reply)
	if reply.Err != nil {
		return errors.New("已经map")
	}
	return nil
}

func CallFinishReduceTask(workerID int, reduceID int) error {
	reply := FinishReply{}
	args := FinishArgs{}
	args.WorkerID = workerID
	args.ID = reduceID
	call("Master.FinishReduceTask", &args, &reply)
	if reply.Err != nil {
		return errors.New("已经reduce")
	}
	return nil
}

func CallDeleteWorker(workerID int) {
	reply := FinishReply{}
	args := FinishArgs{}
	args.WorkerID = workerID
	call("Master.DeleteWorker", &args, &reply)
	if reply.Err != nil {
		log.Fatal(reply)
	}
}

func GetMasterState() int {
	reply := MasterStateReply{}
	args := ExampleArgs{}
	call("Master.GetState", &args, &reply)
	if reply.Err != nil {
		log.Fatal(reply)
	}
	return reply.MasterState
}

// 请求map task
func RequestMapTask(workerID int) NewMapTaskReply {
	args := NewMapTaskArgs{}
	reply := NewMapTaskReply{}

	args.WorkerID = workerID
	call("Master.NewMapTask", &args, &reply)
	if reply.Err != nil {
		fmt.Println(reply.Err)
	} else {
		fmt.Printf("[INFO]得到一个map任务, 文件名:%s  任务编号:%d  nReduce:%d\n", reply.Filename, reply.Id, reply.NReduce)
	}
	return reply
}

// 执行Map任务
func ExecuteMapTask(workerID int, taskInfo NewMapTaskReply, mapf func(string, string) []KeyValue) error {
	//读取文件内容
	contents, err := ioutil.ReadFile(taskInfo.Filename)

	if err != nil {
		return errors.New("[ERROR IN MAP STAGE] IO ERROR")
	}

	// fmt.Printf("File contents: %s", contents)
	kva := mapf(taskInfo.Filename, string(contents))

	// 将产生的<key, value>按照key值分到不同的partition
	// 产生的临时文件名：mr-out-mapId-pid-workID
	enc := make([]*json.Encoder, taskInfo.NReduce)
	for i := 0; i < taskInfo.NReduce; i++ {
		filename := "mr-out-" + strconv.Itoa(taskInfo.Id) + "-" + strconv.Itoa(i) + "-" + strconv.Itoa(workerID) + ".tmp"
		writer, _ := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0755)
		enc[i] = json.NewEncoder(writer)
	}

	// 对kva按照进行排序
	sort.Slice(kva, func(i, j int) bool {
		return kva[i].Key <= kva[j].Key
	})

	// 写到文件
	for _, kv := range kva {
		i := ihash(kv.Key) % taskInfo.NReduce
		if err != nil {
			log.Fatal(err)
		}
		enc[i].Encode(&kv)
	}
	return nil
}

func RequestReduceTask(workerID int) NewReduceTaskReply {
	args := NewReduceTaskArgs{}
	reply := NewReduceTaskReply{}

	args.WorkerID = workerID
	call("Master.NewReduceTask", args, &reply)
	if reply.Err != nil {
		fmt.Println(reply.Err)
	} else {
		fmt.Printf("[INFO]得到一个reduce任务, ReduceId:%d\n", reply.Target)
	}
	return reply
}

// 未调试
func ExecuteReduceTask(workerID int, taskInfo NewReduceTaskReply, reducef func(string, []string) string) error {
	// TODO 方法二：使用多路归并排序

	// sort
	pid := taskInfo.Target
	mapCnts := taskInfo.MapCnts
	mapFiles := []string{}
	for i := 0; i < mapCnts; i++ {
		mapFiles = append(mapFiles, "mr-out-"+strconv.Itoa(i)+"-"+strconv.Itoa(pid))
	}
	all := []KeyValue{}
	//fmt.Println(mapFiles) debug
	for i := 0; i < mapCnts; i++ {
		file, _ := ioutil.ReadFile(mapFiles[i])
		reader := strings.NewReader(string(file))
		dec := json.NewDecoder(reader)
		for {
			m := KeyValue{}
			if err := dec.Decode(&m); err == io.EOF {
				break
			} else if err != nil {
				log.Fatal(err)
			}
			all = append(all, m)
		}
	}

	// 若为空
	if len(all) == 0 {
		return nil
	}
	// 对kva按照进行排序
	sort.Slice(all, func(i, j int) bool {
		return all[i].Key <= all[j].Key
	})

	k0 := all[0].Key
	v := []string{all[0].Value}
	output := []KeyValue{}
	all = all[1:]
	for _, kv := range all {
		if kv.Key != k0 {
			reduced := reducef(k0, v)
			output = append(output, KeyValue{k0, reduced})
			k0 = kv.Key
			v = v[0:0]
		}
		v = append(v, kv.Value)
	}
	reduced := reducef(k0, v)
	output = append(output, KeyValue{k0, reduced})
	// 输出到文件
	targetFile := "mr-out-" + strconv.Itoa(pid) + "-" + strconv.Itoa(workerID) + ".tmp"
	writer, _ := os.OpenFile(targetFile, os.O_RDWR|os.O_CREATE, 0755)
	for _, kv := range output {
		fmt.Fprintf(writer, "%v %v\n", kv.Key, kv.Value)
	}
	writer.Close()

	// 删除临时文件
	for _, filePath := range mapFiles {
		os.Remove(filePath)
	}

	return nil
}

//
// send an RPC request to the master, wait for the response.
// usually returns true.
// returns false if something goes wrong.
//
func call(rpcname string, args interface{}, reply interface{}) bool {
	// c, err := rpc.DialHTTP("tcp", "127.0.0.1"+":1234")
	sockname := masterSock()
	c, err := rpc.DialHTTP("unix", sockname)
	if err != nil {
		log.Fatal("dialing:", err)
	}
	defer c.Close()

	err = c.Call(rpcname, args, reply)
	if err == nil {
		return true
	}

	fmt.Println(err)
	return false
}
