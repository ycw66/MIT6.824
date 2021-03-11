package raft

//
// this is an outline of the API that raft must expose to
// the service (or tester). see comments below for
// each of these functions for more details.
//
// rf = Make(...)
//   create a new Raft server.
// rf.Start(command interface{}) (index, term, isleader)
//   start agreement on a new log entry
// rf.GetState() (term, isLeader)
//   ask a Raft for its current term, and whether it thinks it is leader
// ApplyMsg
//   each time a new entry is committed to the log, each Raft peer
//   should send an ApplyMsg to the service (or tester)
//   in the same server.
//

import (
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"../labrpc"
)

// import "bytes"
// import "../labgob"

//
// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make(). set
// CommandValid to true to indicate that the ApplyMsg contains a newly
// committed log entry.
//
// in Lab 3 you'll want to send other kinds of messages (e.g.,
// snapshots) on the applyCh; at that point you can add fields to
// ApplyMsg, but set CommandValid to false for these other uses.
//

type ServerState int

const (
	Follower  = 0
	Candidate = 1
	Leader    = 2
)

type ApplyMsg struct {
	CommandValid bool
	Command      interface{}
	CommandIndex int
}

//
// A Go object implementing a single Raft peer.
//
type Raft struct {
	mu        sync.Mutex          // Lock to protect shared access to this peer's state
	peers     []*labrpc.ClientEnd // RPC end points of all peers
	persister *Persister          // Object to hold this peer's persisted state
	me        int                 // this peer's index into peers[]
	dead      int32               // set by Kill()

	// Your data here (2A, 2B, 2C).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.
	currentTerm int         //latest term server has seen(init 0)
	votedFor    interface{} //candidateId that received vote in currentTerm(or null if none)
	state       ServerState // indicating the state of server
	waitingTime int         // waiting time for next election
}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	var term int
	var isleader bool
	// Your code here (2A).
	term = rf.currentTerm
	isleader = (rf.state == Leader)

	return term, isleader
}

//
// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
//
func (rf *Raft) persist() {
	// Your code here (2C).
	// Example:
	// w := new(bytes.Buffer)
	// e := labgob.NewEncoder(w)
	// e.Encode(rf.xxx)
	// e.Encode(rf.yyy)
	// data := w.Bytes()
	// rf.persister.SaveRaftState(data)
}

//
// restore previously persisted state.
//
func (rf *Raft) readPersist(data []byte) {
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
	// Your code here (2C).
	// Example:
	// r := bytes.NewBuffer(data)
	// d := labgob.NewDecoder(r)
	// var xxx
	// var yyy
	// if d.Decode(&xxx) != nil ||
	//    d.Decode(&yyy) != nil {
	//   error...
	// } else {
	//   rf.xxx = xxx
	//   rf.yyy = yyy
	// }
}

//
// example RequestVote RPC arguments structure.
// field names must start with capital letters!
//
type RequestVoteArgs struct {
	// Your data here (2A, 2B).
	Term         int //candidate's term
	CandidateId  int //candidate requesting vote
	LastLogIndex int // index of candidate's last log entry
	LastLogTerm  int // term of candidate‘s last log term
}

//
// example RequestVote RPC reply structure.
// field names must start with capital letters!
//
type RequestVoteReply struct {
	// Your data here (2A).
	Term        int  // currentTerm, for candidate to update itself
	VoteGranted bool // true means candidate received vote
}

// TODO: 2B
type AppendEntriesArgs struct {
	Term     int // lead's term
	LeaderId int // so follower can redirect clients
}

// TODO: 2B
type AppendEntriesReply struct {
	Term    int  // currentTerm, for leader to update itself
	Success bool // true if follower contained entry matching preLogIndex and prevLogTerm
}

//
// example RequestVote RPC handler.
//
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (2A, 2B).
	rf.mu.Lock()
	defer rf.mu.Unlock()
	reply.Term = rf.currentTerm
	if rf.currentTerm < args.Term {
		rf.currentTerm = args.Term
		rf.votedFor = nil
		rf.state = Follower
	}
	if rf.state != Follower {
		reply.VoteGranted = false
	} else { // Follower
		rf.waitingTime = 0
		if args.Term < rf.currentTerm {
			reply.VoteGranted = false
			// TODO: log条件 2B
		} else if rf.votedFor == nil || rf.votedFor == args.CandidateId {
			reply.VoteGranted = true
			rf.votedFor = args.CandidateId
			rf.currentTerm = args.Term
			DPrintf("[INFO %d] %d votes for %d\n", rf.me, rf.me, args.CandidateId)
		} else {
			reply.VoteGranted = false
		}
	}
}

// AppendEntries is RPC to implement heatbeats
func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	// TODO : 2B
	// do nothing but reset the waitingTime
	rf.mu.Lock()
	reply.Term = rf.currentTerm
	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		rf.waitingTime = 0
		rf.state = Follower
		rf.votedFor = nil
	} else if args.Term == rf.currentTerm {
		// current server must be a follower
		rf.waitingTime = 0
	}
	rf.mu.Unlock()
}

//
// example code to send a RequestVote RPC to a server.
// server is the index of the target server in rf.peers[].
// expects RPC arguments in args.
// fills in *reply with RPC reply, so caller should
// pass &reply.
// the types of the args and reply passed to Call() must be
// the same as the types of the arguments declared in the
// handler function (including whether they are pointers).
//
// The labrpc package simulates a lossy network, in which servers
// may be unreachable, and in which requests and replies may be lost.
// Call() sends a request and waits for a reply. If a reply arrives
// within a timeout interval, Call() returns true; otherwise
// Call() returns false. Thus Call() may not return for a while.
// A false return can be caused by a dead server, a live server that
// can't be reached, a lost request, or a lost reply.
//
// Call() is guaranteed to return (perhaps after a delay) *except* if the
// handler function on the server side does not return.  Thus there
// is no need to implement your own timeouts around Call().
//
// look at the comments in ../labrpc/labrpc.go for more details.
//
// if you're having trouble getting RPC to work, check that you've
// capitalized all field names in structs passed over RPC, and
// that the caller passes the address of the reply struct with &, not
// the struct itself.
//
func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	// if ok {
	// 	DPrintf("[INFO %d] candidate %d send requestvote to server %d\n", rf.me, rf.me, server)
	// }
	return ok
}

func (rf *Raft) sendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
	return ok
}

//
// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election. even if the Raft instance has been killed,
// this function should return gracefully.
//
// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// term. the third return value is true if this server believes it is
// the leader.
//
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	index := -1
	term := -1
	isLeader := true

	// Your code here (2B).

	return index, term, isLeader
}

//
// the tester doesn't halt goroutines created by Raft after each test,
// but it does call the Kill() method. your code can use killed() to
// check whether Kill() has been called. the use of atomic avoids the
// need for a lock.
//
// the issue is that long-running goroutines use memory and may chew
// up CPU time, perhaps causing later tests to fail and generating
// confusing debug output. any goroutine with a long-running loop
// should call killed() to check whether it should stop.
//
func (rf *Raft) Kill() {
	atomic.StoreInt32(&rf.dead, 1)
	// Your code here, if desired.
}

func (rf *Raft) killed() bool {
	z := atomic.LoadInt32(&rf.dead)
	return z == 1
}

func (rf *Raft) getRandomTimeout() int {
	return (rand.Int()%500 + 300)
}

func (rf *Raft) startAppendEntries() {
	for i := 0; i < len(rf.peers); i++ {
		if i == rf.me {
			continue
		}
		go func(i int) {
			rf.mu.Lock()
			args := AppendEntriesArgs{}
			reply := AppendEntriesReply{}
			args.Term = rf.currentTerm
			args.LeaderId = rf.me
			rf.mu.Unlock()
			if rf.sendAppendEntries(i, &args, &reply) {
				rf.mu.Lock()
				if reply.Term > rf.currentTerm {
					rf.state = Follower
					rf.votedFor = nil
					rf.currentTerm = reply.Term
					rf.waitingTime = 0
				}
				rf.mu.Unlock()
			}
		}(i)
	}
}

func (rf *Raft) startElection() {
	votes := 1
	DPrintf("[INFO test begin%d]\n", rf.me)
	for i := 0; i < len(rf.peers); i++ {
		if i == rf.me {
			continue
		}
		// request concurrently
		go func(i int) {
			// when request concurrently, we should define vars below in every goroutine in order to avoid racing
			requestArgs := RequestVoteArgs{}
			replyArgs := RequestVoteReply{}
			rf.mu.Lock()
			requestArgs.Term = rf.currentTerm
			requestArgs.CandidateId = rf.me
			DPrintf("[Request %d -> %d]\n", rf.me, i)
			rf.mu.Unlock()
			if rf.sendRequestVote(i, &requestArgs, &replyArgs) {
				rf.mu.Lock()
				if replyArgs.VoteGranted {
					votes++
					if 2*votes > len(rf.peers) && rf.state == Candidate {
						// elected as leader
						rf.state = Leader
						rf.waitingTime = 0
						DPrintf("[INFO %d] %d is elected as leader\n", rf.me, rf.me)
					}
				} else {
					// if there is some server whose term is bigger, current server must be a follower
					if replyArgs.Term > rf.currentTerm {
						rf.state = Follower
						rf.currentTerm = replyArgs.Term
					}
				}
				rf.mu.Unlock()
			}
			DPrintf("[Answer %d -> %d]\n", i, rf.me)
		}(i)
	}
}

func (rf *Raft) loop() {
	timeout := rf.getRandomTimeout()
	for {
		if rf.killed() {
			return
		}
		rf.mu.Lock()
		state := rf.state
		rf.mu.Unlock()
		switch state {
		case Follower, Candidate:
			time.Sleep(time.Duration(50) * time.Millisecond)
			//TODO:2B
			rf.mu.Lock()
			rf.waitingTime += 50
			if rf.waitingTime > timeout {
				rf.currentTerm++
				rf.state = Candidate
				rf.waitingTime = 0
				rf.mu.Unlock()
				DPrintf("[INFO %d] timeout=%d\n", rf.me, timeout)
				rf.startElection()
				timeout = rf.getRandomTimeout()
			} else {
				rf.mu.Unlock()
			}

		case Leader:
			rf.startAppendEntries()
			time.Sleep(time.Duration(200) * time.Millisecond)
		}
	}
}

//TODO: adjust the timeout now(300, 600)ms
func (rf *Raft) ticker() {
	for {
		if rf.killed() {
			return
		}
		timeout := rf.getRandomTimeout()
		for {
			if rf.killed() {
				return
			}
			rf.mu.Lock() // lock the rf.waitingTime
			// timer is stoped if server is elected as leader
			if rf.state != Leader { //
				rf.waitingTime += 50
				if rf.waitingTime > timeout {
					rf.state = Candidate
					rf.waitingTime = 0
					rf.currentTerm++
					DPrintf("[INFO %d] timeout=%d\n", rf.me, timeout)
					DPrintf("[INFO %d] server %d change from follower to candidate, term=%d\n", rf.me, rf.me, rf.currentTerm)
					rf.mu.Unlock()
					break
				}
			}
			rf.mu.Unlock()
			time.Sleep(time.Duration(50) * time.Millisecond)
			// DPrintf("[INFO %d] now=%d\n", rf.me, now)
		}
		// timeout elapsed, sendRequestvote
		// state from follower to candidate
		votes := 1
		//TODO:2B
		//requestArgs.LastLogIndex =
		//requestArgs.LastLogTerm =
		DPrintf("[INFO test begin%d]\n", rf.me)
		ch := make(chan int, len(rf.peers)-1)
		for i := 0; i < len(rf.peers); i++ {
			if i == rf.me {
				continue
			}
			// request concurrently
			go func(i int) {

				// when request concurrently, we should define vars below in every goroutine in order to avoid racing
				requestArgs := RequestVoteArgs{}
				replyArgs := RequestVoteReply{}
				requestArgs.Term = rf.currentTerm
				requestArgs.CandidateId = rf.me
				DPrintf("[Request %d -> %d]\n", rf.me, i)
				if rf.sendRequestVote(i, &requestArgs, &replyArgs) {
					rf.mu.Lock()
					if replyArgs.VoteGranted {
						votes++
						if 2*votes > len(rf.peers) && rf.state == Candidate {
							// elected as leader
							rf.state = Leader
							DPrintf("[INFO %d] %d is elected as leader\n", rf.me, rf.me)
						}
					} else {
						// if there is some server whose term is bigger, current server must be a follower
						if replyArgs.Term > rf.currentTerm {
							rf.state = Follower
							rf.currentTerm = replyArgs.Term
						}
					}
					rf.mu.Unlock()
				}
				DPrintf("[Answer %d -> %d]\n", i, rf.me)
				ch <- i
			}(i)
		}
		// waiting all the result is processed
		for i := 0; i < len(rf.peers)-1; i++ {
			<-ch
		}
		DPrintf("[INFO test end%d]\n", rf.me)

		rf.mu.Lock()
		// reset to follower
		if rf.state != Leader {
			rf.state = Follower
		}
		rf.mu.Unlock()
	}
}

// check current server if is the leader. If yes, send heartbeat to all followers
func (rf *Raft) heartBeat() {
	for {
		if rf.killed() {
			return
		}
		rf.mu.Lock()
		state := rf.state
		rf.mu.Unlock()
		if state == Leader {
			//TODO: 并发改进
			for i := 0; i < len(rf.peers); i++ {
				if i == rf.me {
					continue
				}
				go func(i int) {
					rf.mu.Lock()
					args := AppendEntriesArgs{}
					reply := AppendEntriesReply{}
					args.Term = rf.currentTerm
					args.LeaderId = rf.me
					rf.mu.Unlock()
					if rf.sendAppendEntries(i, &args, &reply) {
						rf.mu.Lock()
						if reply.Term > rf.currentTerm {
							rf.state = Follower
							rf.votedFor = nil
							rf.currentTerm = reply.Term
							rf.waitingTime = 0
						}
						rf.mu.Unlock()
					}
				}(i)
			}
		}
		time.Sleep(time.Duration(200) * time.Millisecond)
	}
}

//
// the service or tester wants to create a Raft server. the ports
// of all the Raft servers (including this one) are in peers[]. this
// server's port is peers[me]. all the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// tester or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
//
func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me
	rf.currentTerm = 0
	rf.state = Follower
	rf.votedFor = nil
	rf.waitingTime = 0
	// Your initialization code here (2A, 2B, 2C).
	// go rf.ticker()
	// go rf.heartBeat()
	go rf.loop()

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	return rf
}
