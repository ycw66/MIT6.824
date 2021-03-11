# Lab2	Raft

## 介绍

Raft是一个分布式共识协议，是我们构建一个高容错的分布式数据库的第一步。

为了实现容错，我们采用多副本策略，为一个文件设置几个不同的副本分别放置到不同的机器上，当其中一个出错后，我们可以根据副本中的内容进行错误恢复。但是，引入多副本策略同时也带来了一致性问题，而这次实验的主角Raft就是为解决此问题而生的！

容易理解的动画：https://thesecretlivesofdata.com/raft/



如何理解最终一致性？与强一致性的区别是什么？



Raft将一致性维护算法分为三个步骤：

1. leader选举
2. 日志备份
3. 安全性



依赖包

- `labrpc`	用来模拟真实网络错误

- `testing`  用来测试

## Part 2A: leader election

### 提示

1. 关注论文raft-extend中图2与选举有关的内容
   1. 在Make()方法创建一个后台周期请求选举的进程，随机时间500ms-1000ms





### ==注意==

1. 候选人不可以投票给其他人

2. leader当选后停止计时器

3. 避免投票和成为candidate同时发生，使用mutex保证

4. 在disconnect后Call返回失败的时间较长，此时如果主动等待其返回结果，会造成其他的follower进化成candidate，所以不能选择主动等待结果。（这个BUG卡我最久）



为什么go语言编译器无法发现同一个锁的嵌套使用

```go
// 错误例子
func() A{
a.lock()
B()
a.unlock()
}

func() B {
a.lock()
a.unlock()

}
```





#### 错误日志

1. 对应注意点(4)

   ```bash
   Test (2A): initial election ...
   2021/03/11 20:27:45 [INFO 1] timeout=1755
   2021/03/11 20:27:45 [INFO 1] server 1 change from follower to candidate, term=1
   2021/03/11 20:27:45 [INFO 2] timeout=1797
   2021/03/11 20:27:45 [INFO 2] server 2 change from follower to candidate, term=1
   2021/03/11 20:27:45 [INFO test begin1]
   2021/03/11 20:27:45 [INFO test begin2]
   2021/03/11 20:27:45 [Request 1 -> 0]
   2021/03/11 20:27:45 [Request 2 -> 0]
   2021/03/11 20:27:45 [INFO 0] 0 votes for 1
   2021/03/11 20:27:45 [Answer 0 -> 1]
   2021/03/11 20:27:45 [Request 1 -> 2]
   2021/03/11 20:27:45 [Answer 0 -> 2]
   2021/03/11 20:27:45 [Request 2 -> 1]
   labgob warning: Decoding into a non-default variable/field VoteGranted may not work
   2021/03/11 20:27:45 [Answer 2 -> 1]
   2021/03/11 20:27:45 [INFO test end1]
   2021/03/11 20:27:45 [INFO 1] 1 is elected as leader
   2021/03/11 20:27:45 [Answer 1 -> 2]
   2021/03/11 20:27:45 [INFO test end2]
     ... Passed --   4.5  3   32    4048    0
   Test (2A): election after network failure ...
   2021/03/11 20:27:49 [INFO 0] timeout=1589
   2021/03/11 20:27:49 [INFO 0] server 0 change from follower to candidate, term=1
   2021/03/11 20:27:49 [INFO test begin0]
   2021/03/11 20:27:49 [Request 0 -> 1]
   2021/03/11 20:27:49 [INFO 1] 1 votes for 0
   2021/03/11 20:27:49 [Answer 1 -> 0]
   2021/03/11 20:27:49 [Request 0 -> 2]
   2021/03/11 20:27:49 [INFO 2] 2 votes for 0
   2021/03/11 20:27:49 [Answer 2 -> 0]
   2021/03/11 20:27:49 [INFO test end0]
   2021/03/11 20:27:49 [INFO 0] 0 is elected as leader
   2021/03/11 20:27:49 [INFO TEST] 0 is disconnected
   2021/03/11 20:27:52 [INFO 1] timeout=2191
   2021/03/11 20:27:52 [INFO 1] server 1 change from follower to candidate, term=2
   2021/03/11 20:27:52 [INFO test begin1]
   2021/03/11 20:27:52 [Request 1 -> 0]
   2021/03/11 20:27:52 [INFO 2] timeout=2230
   2021/03/11 20:27:52 [INFO 2] server 2 change from follower to candidate, term=2
   2021/03/11 20:27:52 [INFO test begin2]
   2021/03/11 20:27:52 [Request 2 -> 0]
   --- FAIL: TestReElection2A (6.90s)
       config.go:330: expected one leader, got none
   FAIL
   exit status 1
   FAIL	_/home/weggle/programming/6.824/src/raft	11.408s
   ```

   