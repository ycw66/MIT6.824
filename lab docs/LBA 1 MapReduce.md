# LBA 1 MapReduce 实验报告

- MR组成：一个main process 多个 worker process

- 任务：结合paper，完成master和worker，实现mapreduce框架

- 介绍：`pg-*.txt`是输入文件，一个文件代表一个`split`

## **要求**

- map阶段要将产生的中间结果<key, list< value >> 根据key分到不同的桶，共有`nReduce`reduce任务
- `worker`将第x个reduce任务的输出结果保存到文件`mr-out-x`中
- 每一个`mr-out-x`应该包含对应的Reduce函数的所有输出，每一个Reduce函数输出一行，输出格式为`"%v %v"`
- 最终版本只需要更改`worker.go` `master.go` `rpc.go` 这三个文件
- `worker` 将Map函数输出的中间结果保存到当前文件夹(worker启动的文件夹)，后面的reduce 任务需要从这里读取文件
- `main/mrmaster.go` 期望 `mr/master.go`实现一个`Done()`方法，当整个MapReduce工作完成后，`Done()`方法返回真，此时`main/mrmaster`将退出
- 当MapReduce工作完成后，所有的`worker`要退出

## **提示**

- 一个开始的方式是修改`mr/worker.go`的`Worker()`方法，通过RPC向`master`请求一个任务。然后修改`mr/master.go`能够返回一个文件名，此文件作为一个map 任务的输入数据。然后修改`mr/worker.go`，使`worker`读取该文件的内容并调用应用程序的Map函数，应用程序的Map和Reduce函数通过`Go`的 plugin package来获得， 像`mrsequential.go`里写的那样。

- 真实的MapReduce依赖于分布式文件系统，但在我们这个实验中，所有的`master`

  和`worker`都运行在一个机器上，使用本地机器的文件系统即可。

- 一个合理的中间文件的命名方式为：`mr-X-Y`，其中X为map任务的编号，Y为reduce任务的编号。

- 为了方便reduce 任务读取<key, value>中间结果，使用`encoding/json`读写文件

  - 写文件（map task）

  ```go
  enc := json.NewEncoder(file)
  for _, kv := ... {
      err := enc.Encode(&kv)
  ```

  - 读文件（reduce task）

  ```go
  dec := json.NewDecoder(file)
  for {
      var kv KeyValue
      if err := dec.Decode(&kv); err != nil {
      	break
      }
      kva = append(kva, kv)
  }
  ```

- `work.go` 中的`ihash`方法可以方便map task 为key选择对应的reduce task

- 可以从`mrsequential.go`中copy一部分代码

- `master`是一个RPC服务器，注意锁上共享数据

- 可以使用`Go`的`race detector`检查有无`race`

  `go build -race` 	`go run -race. test-mr.sh`

- `master`何时判定一个`worker`坏掉了，`master`等待`worker`的执行信息，10秒未接收到，就认为`worker`挂掉了

- 如果想要测试恢复机制，可以使用`mrapps/crash.go`的应用插件，它可以随机退出工作中的`Map`和`Reduce`函数

- 可以使用`ioutil.TempFile`创建一个临时文件，通过`os.Rename`重命名它
- `test-mr.sh`将运行结果保存在文件夹`mr-tmps`中

## 流程图

![image-20210125215435798](/home/weggle/.config/Typora/typora-user-images/image-20210125215435798.png)



命令：

`master`

```bash
go build -buildmode=plugin ../mrapps/wc.go
go run -race mrmaster.go pg*.txt
rm mr-
```

`worker`

```bash
go run -race mrworker.go wc.so
```





## **设计细节**

1. worker创建完毕，首先向master进行注册。

   ```go
   // worker.go
   func Worker(mapf func(string, string) []KeyValue,
   	reducef func(string, []string) string) {
   	RegiserInfo := RegisterWork()
   }
   ```

   

2. worker在空闲状态时，获取master的状态信息，master返回当前整个MapReduce任务的执行状态，下面根据回复信息，work向master请求相应的任务。

   | masterState | 意义                             |
   | ----------- | -------------------------------- |
   | 1           | 有剩余的map task                 |
   | 2           | Map阶段结束，有剩余的reduce task |
   | 3           | 整个MapReduce任务结束            |
   | 其他        | 需要等待                         |

   ```go
   // worker.go
   masterState = GetMasterState()
   if masterState  == 1 {
       // 请求Map任务
   } else masterState == 2 {
       // 请求Reduce任务
   } else masterState == 3 {
       // 请求退出
   } else {
       // 等待
   }
   ```

3. master维护所有注册的worker的信息，并每隔10s判断他们的执行状态

   ```go
   // master.go
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
   ```

   - 保证如果10s内worker没有执行完相应的任务，就将此任务回收

   

4. worker执行一个任务的流程如下：请求任务，执行任务，任务完成后向master的反馈。master会根据worker的反馈结果更新mapreduce任务的进度。

   ```go
   // worker.go
   // 以执行Map任务为例
   if masterState == 1 {
       taskInfo := RequestMapTask(workerID)
       ExecuteMapTask(workerID, taskInfo, mapf)
       CallFinishMapTask(workerID, taskInfo.Id, taskInfo.Filename) 
   }
   ```

5. worker和master之间采用RPC进行通信，注意对master中的方法加锁以避免竞争冒险。



>实验详细代码：
>
>https://github.com/ycw66/MIT6.824/tree/master/src/main



