package mr

//
// RPC definitions.
//
// remember to capitalize all names.
//

import (
	"os"
	"strconv"
)

// ExampleArgs is
type ExampleArgs struct {
	X int
}

// ExampleReply is
type ExampleReply struct {
	Y int
}

type EmptyArgs struct {
}

// RegisterReply is
type RegisterReply struct {
	MasterState int   //0:no extra task 1：map stage 2: reduce stage
	WorkerID    int   //workers'id, unique
	Err         error //indicate if there is an error
}

type MasterStateReply struct {
	MasterState int
	Err         error
}

type NewMapTaskArgs struct {
	WorkerID int
}

type NewMapTaskReply struct {
	Filename string //file name
	Id       int    //map-task id
	NReduce  int    //total number of reduce task
	Err      error  //indicate if there is an error
}

type NewReduceTaskArgs struct {
	WorkerID int
}

type NewReduceTaskReply struct {
	Target  int //目标reduce任务 范围：[0, nReduce)
	MapCnts int //Map task 的总数
	Err     error
}

type FinishArgs struct {
	Filename string
	WorkerID int
	ID       int
}
type FinishReply struct {
	Err error
}

// Cook up a unique-ish UNIX-domain socket name
// in /var/tmp, for the master.
// Can't use the current directory since
// Athena AFS doesn't support UNIX-domain sockets.
func masterSock() string {
	s := "/var/tmp/824-mr-"
	s += strconv.Itoa(os.Getuid())
	return s
}
