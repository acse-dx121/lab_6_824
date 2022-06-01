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

// for Lab 2A, some information:
//		time required: 1. heartbeat RPCs no more than ten times per second
//									 2. elect a new leader within five seconds
//									 3. election timeout due to the limitaion of frequency heartbeat RPC.

import (
	//	"bytes"
	"sync"
	"sync/atomic"
	"time"

	//	"6.824/labgob"
	"6.824/labrpc"
)

//
// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make(). set
// CommandValid to true to indicate that the ApplyMsg contains a newly
// committed log entry.
//
// in part 2D you'll want to send other kinds of messages (e.g.,
// snapshots) on the applyCh, but set CommandValid to false for these
// other uses.
//
type ApplyMsg struct {
	CommandValid bool
	Command      interface{}
	CommandIndex int

	// For 2D:
	SnapshotValid bool
	Snapshot      []byte
	SnapshotTerm  int
	SnapshotIndex int
}

// role defination
const follower int = 1
const candidate int = 2
const leader int = 3

var role_map map[int]string

//
// A Go object implementing a single Raft peer.
//
type Raft struct {
	mu        sync.Mutex          // Lock to protect shared access to this peer's state
	cond      *sync.Cond          //
	peers     []*labrpc.ClientEnd // RPC end points of all peers
	persister *Persister          // Object to hold this peer's persisted state
	me        int                 // this peer's index into peers[]
	dead      int32               // set by Kill()

	// Your data here (2A, 2B, 2C).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.

	// State
	role     int // 1: follower 2: candidate 3: leader
	leaderId int // leaderId == me ? decide if me is leader

	// Persistent States
	currentTerm int     // latest term server has seen
	voteFor     int     // index of peers, candidatedId that received vote in current term
	Log         []Entry // contain commands with log index and term

	// volatile state
	commitIndex int // index of highest log entry known to be commited
	lastApplied int //

	// volatile state for leaders
	nextIndex  []int
	matchIndex []int

	// when follower received AppendEntries rpc, send a message to channel. TODO: maybe block.
	heartbeat chan string
	applyCh   chan ApplyMsg
}

type Entry struct {
	Command interface{}
	Term    int
}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {

	// var term int
	// var isleader bool
	// Your code here (2A).

	rf.mu.Lock()
	defer rf.mu.Unlock()

	return rf.currentTerm, rf.role == leader
}

// state transfer
// beLeader clear the nextIndex and matchIndex
func (rf *Raft) beLeaer() {
	DPrintf("%d become leader from %s. Term: %d", rf.me, role_map[rf.role], rf.currentTerm)
	rf.role = leader

	rf.nextIndex = make([]int, len(rf.peers))
	rf.matchIndex = make([]int, len(rf.peers))
	for i := 0; i < len(rf.peers); i++ {
		rf.matchIndex[i] = 0
		rf.nextIndex[i] = 1
	}

}

// beCandidate
func (rf *Raft) beCandidate() {
	DPrintf("%d become candidate from %s. Term: %d", rf.me, role_map[rf.role], rf.currentTerm)
	rf.role = candidate
}

// beFollower
func (rf *Raft) beFollower() {
	DPrintf("%d become follower from %s. Term: %d", rf.me, role_map[rf.role], rf.currentTerm)

	rf.updateVoteFor(-1)
	// rf.voteFor = -1 // voteFor should be reset when transfer to followers
	rf.role = follower
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
// A service wants to switch to snapshot.  Only do so if Raft hasn't
// have more recent info since it communicate the snapshot on applyCh.
//
func (rf *Raft) CondInstallSnapshot(lastIncludedTerm int, lastIncludedIndex int, snapshot []byte) bool {

	// Your code here (2D).

	return true
}

// the service says it has created a snapshot that has
// all info up to and including index. this means the
// service no longer needs the log through (and including)
// that index. Raft should now trim its log as much as possible.
func (rf *Raft) Snapshot(index int, snapshot []byte) {
	// Your code here (2D).

}

//
// example RequestVote RPC arguments structure.
// field names must start with capital letters!
//
type RequestVoteArgs struct {
	// Your data here (2A, 2B).
	Term         int // candidate's Term
	CandidateId  int // candidate requesting vote
	LastLogIndex int // index of candidate's last log entry
	LastLogTerm  int // term of candidate's last log entry
}

//
// example RequestVote RPC reply structure.
// field names must start with capital letters!
//
type RequestVoteReply struct {
	// Your data here (2A).
	Term        int  // current Term, for candidate to update itsekf
	VoteGranted bool // true means candidate received vote
}

// update the currentTerm
func (rf *Raft) updateCurrentTerm(term int) {
	rf.currentTerm = term
	rf.persist()
}

// update the voteFor
func (rf *Raft) updateVoteFor(candidateId int) {
	rf.voteFor = candidateId
	rf.persist()
}

// update logs
func (rf *Raft) updateLogs(index int, entries []Entry) {
	rf.Log = append(rf.Log[:index], entries...)
	rf.persist()
}

// startElection
func (rf *Raft) startElection() {

	rf.mu.Lock()
	// change the role from follower to candidate
	if rf.role == follower {
		rf.beCandidate()
	}

	// currentTerm add more one
	rf.updateCurrentTerm(rf.currentTerm + 1)
	// vote for himself
	rf.updateVoteFor(rf.me)
	// pack the args
	currentTerm, role := rf.currentTerm, rf.role
	rf.mu.Unlock()

	// init result reply
	reply := make([]*RequestVoteReply, len(rf.peers))
	for i := 0; i < len(rf.peers); i++ {
		reply[i] = &RequestVoteReply{}
	}

	voteCount := 1
	finished := 0
	for i := 0; i < len(rf.peers); i++ {
		if i == rf.me { // skip myself
			continue
		}

		DPrintf("Machine %d send request vote to %d. Term: %d", rf.me, i, rf.currentTerm)
		go func(index int, reply *RequestVoteReply) {
			rf.mu.Lock()
			args := &RequestVoteArgs{}
			args.Term = rf.currentTerm
			args.LastLogIndex = len(rf.Log) - 1
			args.LastLogTerm = rf.Log[args.LastLogIndex].Term
			args.CandidateId = rf.me
			rf.mu.Unlock()

			waitChan := make(chan bool)
			go func() {
				start := time.Now()
				ok := rf.sendRequestVote(index, args, reply)
				end := time.Now()
				if end.Sub(start) < time.Millisecond*150 {
					waitChan <- ok
				}
			}()

			ok := false
			select {
			case ok = <-waitChan:
			case <-time.After(time.Millisecond * 150):
			}

			rf.mu.Lock()
			defer rf.mu.Unlock()
			defer rf.cond.Broadcast()

			DPrintf("Machine %d get the reply when sendRequestVote from %d, Status: %v. Term: %d", rf.me, index, ok, rf.currentTerm)
			finished++

			if rf.currentTerm != currentTerm || rf.role != role {
				DPrintf("Machine %d, expired process, currentTerm: %d, role: %d, rf.currentTerm: %d, rf.role: %d", rf.me, currentTerm, role, rf.currentTerm, rf.role)
				return
			}

			if !ok {
				return
			}

			if reply.Term > rf.currentTerm {
				rf.currentTerm = reply.Term
				rf.beFollower()
			}
			if reply.VoteGranted {
				voteCount++
			}
		}(i, reply[i])

	}

	rf.mu.Lock()
	for voteCount <= len(rf.peers)/2 && finished != len(rf.peers)-1 {
		rf.cond.Wait()
	}
	// win the election and change role to the leader
	if voteCount > len(rf.peers)/2 {
		rf.beLeaer()
	}
	rf.mu.Unlock()

	DPrintf("Machine %d has finished the election. Term: %d", rf.me, rf.currentTerm)
}

//
// example RequestVote RPC handler.
//
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (2A, 2B).
	// pack the args
	// DPrintf("Machine %d try to get locker in Requestvote, called by %d. Term: %d", rf.me, args.CandidateId, rf.currentTerm)
	rf.mu.Lock()
	defer rf.mu.Unlock()
	defer func() { rf.heartbeat <- "" }()
	// DPrintf("Machine %d has got locker in Requestvote, called by %d. Term: %d", rf.me, args.CandidateId, rf.currentTerm)

	DPrintf("Machine %d receive request vote from %d. Term: %d", rf.me, args.CandidateId, rf.currentTerm)
	var voter = func(granted bool) {
		reply.VoteGranted = granted
		reply.Term = rf.currentTerm
		DPrintf("Machine %d Vote info - args Term: %d, currentTerm : %d, args.LastLogTerm: %d, args.LastLogIndex:%d, voteFor: %d, CandidateID%d, vote: %v, rf.logs:%v",
			rf.me, args.Term, rf.currentTerm, args.LastLogTerm, args.LastLogIndex, rf.voteFor, args.CandidateId, reply.VoteGranted, rf.Log)
	}

	if args.Term < rf.currentTerm {
		DPrintf("Machine %d reject the request vote from %d because Term < currentTerm. Term: %d", rf.me, args.CandidateId, rf.currentTerm)
		voter(false)
		return
	}

	if args.Term > rf.currentTerm {
		rf.updateCurrentTerm(args.Term)
		// rf.currentTerm = args.Term
		rf.beFollower()
	}

	if rf.voteFor != -1 && rf.voteFor != args.CandidateId {
		DPrintf("Machine %d reject the request vote from %d because voteFor != args.CandidateId. Term: %d", rf.me, args.CandidateId, rf.currentTerm)
		voter(false)
		return
	}

	// TODO: check if the log is the same
	if args.LastLogTerm < rf.Log[len(rf.Log)-1].Term {
		voter(false)
		return
	}

	if args.LastLogTerm == rf.Log[len(rf.Log)-1].Term && args.LastLogIndex < len(rf.Log)-1 {
		DPrintf("Machine %d reject the request vote from %d because log is not the same. Term: %d", rf.me, args.CandidateId, rf.currentTerm)
		voter(false)
		return
	}

	// vote for the candidate
	// DPrintf("Machine %d vote for the candidate %d. Term: %d", rf.me, args.CandidateId, rf.currentTerm)
	rf.updateVoteFor(args.CandidateId)
	// rf.voteFor = args.CandidateId
	voter(true)
	// rf.heartbeat <- ""
	DPrintf("Machine %d finished Requestvote, called by %d. Term: %d", rf.me, args.CandidateId, rf.currentTerm)
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
	return ok
}

//
type AppendEntriesArgs struct {
	Term         int // leader's term
	LeaderId     int // follower can redirect client
	PrevLogIndex int // index of log entry immediately preceding new ones

	PrevLogTerm int     // term of preLogIndex entry
	Entries     []Entry // batch to store commands

	LeaderCommit int // leader's commitIndex
}

//
type AppendEntriesReply struct {
	Term    int  // currentTerm, for leader to update itself
	Success bool // true if follower matching
}

//
func (rf *Raft) sendAERpcs() {
	// DPrintf("machine %d package the args and reply when SendAERpcs. Term: %d", rf.me, rf.currentTerm)
	replys := make([]*AppendEntriesReply, len(rf.peers))
	for i := 0; i < len(rf.peers); i++ {
		replys[i] = &AppendEntriesReply{}
	}

	//
	// DPrintf("machine %d start to send the requests when SendAERpcs. Term: %d", rf.me, rf.currentTerm)
	finished := 0
	for i := 0; i < len(rf.peers); i++ {
		if i == rf.me {
			continue
		}

		//
		DPrintf("machine %d send the request to %d when SendAERpcs. Term: %d", rf.me, i, rf.currentTerm)
		go func(index int, reply *AppendEntriesReply) {
			rf.mu.Lock()
			args := &AppendEntriesArgs{}
			args.LeaderId = rf.me
			args.LeaderCommit = rf.commitIndex
			args.Term = rf.currentTerm

			// prevLogIndex is the nextindex of log, which should be sented
			args.PrevLogIndex = rf.nextIndex[index] - 1
			if args.PrevLogIndex != -1 {
				args.PrevLogTerm = rf.Log[args.PrevLogIndex].Term
			}

			isHeartBeat := rf.nextIndex[index] == len(rf.Log)
			if !isHeartBeat {
				// P2b("%d, %d ", rf.nextIndex[index], len(rf.Log))
				P2b("Machine %d think it is the leader and send the entris to the follower %d, the index of log is %d. Term: %d",
					rf.me, index, rf.matchIndex[index], rf.currentTerm)
				args.Entries = append(args.Entries, rf.Log[rf.nextIndex[index]:]...)
			}
			rf.mu.Unlock()

			waitChan := make(chan bool)
			go func() {
				start := time.Now()
				ok := rf.sendAppendEntries(index, args, reply)
				end := time.Now()
				if end.Sub(start) < time.Millisecond*150 {
					waitChan <- ok
				}
			}()

			ok := false
			select {
			case ok = <-waitChan:
				DPrintf("machine %d get the response of sendAppendEntriews from %d success. Term: %d", rf.me, index, rf.currentTerm)
			case <-time.After(time.Millisecond * 150):
				DPrintf("machine %d get the response of sendAppendEntriews from to %d failed. Term: %d", rf.me, index, rf.currentTerm)
			}

			rf.mu.Lock()
			defer rf.mu.Unlock()
			defer rf.cond.Broadcast()

			DPrintf("machine %d receive the reply from %d when sendAppendEntries, Status: %v. Term: %d", rf.me, index, ok, rf.currentTerm)
			finished++

			if !ok {
				// DPrintf("machine %d get the response to %d failed when SendAERpcs. Term: %d", rf.me, index, rf.currentTerm)
				return
			}

			// if rf.role == follower {
			// 	// DPrintf("Machine %d used to be a leader, but now is follower. Term: %d", rf.me, rf.currentTerm)
			// 	return
			// }

			if reply.Term > rf.currentTerm {
				P2b("machine %d get the response of sendAppendEntries from %d, but the term is %d, so it is not the leader. Term: %d", rf.me, index, reply.Term, rf.currentTerm)
				rf.updateCurrentTerm(reply.Term)
				// rf.currentTerm = reply.Term
				rf.beFollower()
				return
			}

			if isHeartBeat {
				return
			}

			// if success, means all log entries has been appended
			if reply.Success {
				rf.nextIndex[index] += len(args.Entries)
				rf.matchIndex[index] = rf.nextIndex[index] - 1
			} else {
				rf.nextIndex[index]--
			}

		}(i, replys[i])
	}

	// process the result
	rf.mu.Lock()
	for finished != len(rf.peers)-1 {
		rf.cond.Wait()
	}

	//
	count := 1
	for i := 0; i < len(rf.peers); i++ {
		if i != rf.me && rf.nextIndex[i] > rf.commitIndex+1 {
			count++
		}
	}

	DPrintf("machine %d has nextIndex;%v, rf.commitIndex: %d. count:%d,  Term: %d", rf.me, rf.nextIndex, rf.commitIndex, count, rf.currentTerm)
	if count > len(rf.peers)/2 {
		rf.commitIndex++
		P2b("Machine %d send the applyMsg %v, %d. Term: %d", rf.me, rf.Log[rf.commitIndex].Command, rf.commitIndex, rf.currentTerm)
		rf.applyCh <- ApplyMsg{Command: rf.Log[rf.commitIndex].Command, CommandIndex: rf.commitIndex, CommandValid: true}
	}

	rf.mu.Unlock()
	DPrintf("machine %d finish sending the requests when SendAERpcs. Term: %d", rf.me, rf.currentTerm)
}

//
func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	DPrintf("machine %d get the request from %d when AppendEntries. Term: %d", rf.me, args.LeaderId, rf.currentTerm)

	// DPrintf("machine %d try to get the locker when AppendEntries. Term: %d", rf.me, rf.currentTerm)
	rf.mu.Lock()
	defer rf.mu.Unlock()
	// DPrintf("machine %d has got the locker when AppendEntries. Term: %d", rf.me, rf.currentTerm)

	//
	reply.Term = rf.currentTerm
	reply.Success = false

	//
	if args.Term < rf.currentTerm {
		DPrintf("machine %d get a lower term when AppendEntries, the request is not admitted. Term: %d", rf.me, rf.currentTerm)
		return
	}

	//
	if args.Term > rf.currentTerm {
		DPrintf("machine %d get a higher term when AppendEntries, tansfer to the follower. Term: %d", rf.me, rf.currentTerm)
		rf.updateCurrentTerm(args.Term)
		// rf.currentTerm = args.Term
		rf.beFollower()
	}

	//
	if args.PrevLogIndex < 0 || args.PrevLogIndex >= len(rf.Log) {
		DPrintf("machine %d get a wrong precLogIndex when AppendEntries. Term: %d", rf.me, rf.currentTerm)
		return
	}

	//
	if args.PrevLogTerm != rf.Log[args.PrevLogIndex].Term {
		DPrintf("machine %d get a wrong prevLogTerm when AppendEntries. Term: %d", rf.me, rf.currentTerm)
		return
	}

	//
	if len(args.Entries) > 0 {
		P2b("Machine %d is %v, append the logs: %v. Term: %d", rf.me, role_map[rf.role], args.Entries, rf.currentTerm)
		// rf.Log = append(rf.Log[:args.PrevLogIndex+1], args.Entries...)
		rf.updateLogs(args.PrevLogIndex+1, args.Entries)
	}

	//
	if args.LeaderCommit > rf.commitIndex {
		old_commitIndex := rf.commitIndex
		rf.commitIndex = min(args.LeaderCommit, len(rf.Log)-1)

		for i := old_commitIndex + 1; i <= rf.commitIndex; i++ {
			P2b("Machine %d send the applyMsg %v, %d, old_commitIndex: %d, rf.commitIndex: %d. Term: %d", rf.me, rf.Log[i].Command, i, old_commitIndex, rf.commitIndex, rf.currentTerm)
			rf.applyCh <- ApplyMsg{
				Command:      rf.Log[i].Command,
				CommandIndex: i,
				CommandValid: true,
			}
		}
	}

	//
	reply.Success = true
	DPrintf("machine %d send the reply to %d of success msg when AppendEntries. Term: %d", rf.me, args.LeaderId, rf.currentTerm)

	DPrintf("machine %d send the heartbeat when %d when AppendEntries. Term: %d", rf.me, args.LeaderId, rf.currentTerm)
	rf.heartbeat <- ""
	DPrintf("machine %d has sended the heartbeat when %d when AppendEntries. Term: %d", rf.me, args.LeaderId, rf.currentTerm)
}

//
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

	// Your code here (2B).
	rf.mu.Lock()
	defer rf.mu.Unlock()

	// init the status
	index := -1
	term := rf.currentTerm
	isLeader := rf.role == leader

	// if the server think it is the leader
	if isLeader {
		// append the command to the end of logs
		index = len(rf.Log)
		// rf.Log = append(rf.Log, Entry{Term: term, Command: command})
		rf.updateLogs(len(rf.Log), []Entry{{Term: term, Command: command}})
		P2b("Machine %d is the leader, append the log in index %d, Term: %d", rf.me, index, rf.currentTerm)
	}

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

// The ticker go routine starts a new election if this peer hasn't received
// heartsbeats recently.
func (rf *Raft) ticker() {
	DPrintf("%d ticker start", rf.me)

	for rf.killed() == false {
		// Your code here to check if a leader election should
		// be started and to randomize sleeping time using
		// time.Sleep().

		// leader send heartbeat to followers
		if rf.role == leader {
			DPrintf("%d ticker send heartbeat. Term: %d", rf.me, rf.currentTerm)
			rf.sendAERpcs()

			time.Sleep(time.Millisecond * 80)
			continue
		}

		// follower starts a new election or process the heartbeat
		electionTimeout := RandomTime()
		DPrintf("Machine %d has eletiontime: %d. Term: %d", rf.me, electionTimeout/time.Millisecond, rf.currentTerm)
		select {
		case <-time.After(electionTimeout):
			// start election
			DPrintf("Machine: %d, election happend timeout, timeout: %d, role: %s. Term: %d", rf.me, electionTimeout/time.Millisecond, role_map[rf.role], rf.currentTerm)
			rf.startElection()
		case <-rf.heartbeat:
			// print the messages
			DPrintf("Machine: %d refresh the eletiontimeout. Term: %d", rf.me, rf.currentTerm)
			// process the message
		}
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

	DPrintf("Machine: %d, Make()", me)
	role_map = map[int]string{1: "follower", 2: "candidate", 3: "leader", -1: "none"}
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me
	rf.cond = sync.NewCond(&rf.mu)

	// Your initialization code here (2A, 2B, 2C).
	rf.currentTerm = 0
	// when init, no vote for any machine, so init as -1
	rf.voteFor = -1

	rf.Log = make([]Entry, 0)
	rf.Log = append(rf.Log, Entry{Command: nil, Term: 0})
	rf.commitIndex = -1

	rf.leaderId = -1
	rf.lastApplied = -1

	// init nextIndex as the 0 + 1
	rf.nextIndex = make([]int, len(rf.peers))
	rf.matchIndex = make([]int, len(rf.peers))
	for i := 0; i < len(rf.peers); i++ {
		rf.matchIndex[i] = 0
		rf.nextIndex[i] = 1
	}

	rf.heartbeat = make(chan string, 1)
	rf.applyCh = applyCh

	// init as the follower
	rf.mu.Lock()
	rf.beFollower()
	rf.mu.Unlock()

	// applyCh <- ApplyMsg{Command: nil, CommandIndex: 0, CommandValid: true}
	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	// start ticker goroutine to start elections

	go rf.ticker()

	return rf
}
