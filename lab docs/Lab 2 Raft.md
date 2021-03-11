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





### 注意

- 候选人不可以投票给其他人
- leader当选后停止计时器
- 避免投票和成为candidate同时发生，使用mutex保证
- 在disconnect后Call返回失败的时间较长，此时如果主动等待其返回结果，会造成其他的follower进化成candidate，所以不能选择主动等待结果。（这个BUG卡我的时间最久）



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

