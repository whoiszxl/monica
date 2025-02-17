package list

import (
	"Monica/go-yedis/core"
	"strconv"
)

//基础的push命令，给其他命令做调用
//准备是用Redis3.2版本的PushGenericCommand，3.2是用的是quicklist，3.0采用的是双向链表(LinkedList)和压缩列表(ZipList)
//此处改变为使用3.2版本的，但是底层数据结构只使用双向链表空间优化后续优化成QuickList
//Redis3.2代码：https://github.com/antirez/redis/blob/3.2/src/t_list.c#L197
func PushGenericCommand(c *core.YedisClients, s *core.YedisServer, where int) {
	var waiting, pushed = 0, 0
	//搜索key是否存在数据库中
	lobj := core.LookupKey(c.Db.Dict, c.Argv[1])

	if lobj != nil && lobj.ObjectType != core.REDIS_LIST {
		//TODO 错误回复应该在Yedis初始化的时候创建一个共享对象，然后将提示语统一管理,代码地址：https://github.com/huangz1990/redis-3.0-annotated/blob/8e60a75884e75503fb8be1a322406f21fb455f67/src/redis.c#L1613
		core.AddReplyStatus(c, "-WRONGTYPE Operation against a key holding the wrong kind of value\r\n")
		return
	}

	//遍历输入的参数并添加到列表哦
	for i:=2; i<c.Argc; i++ {
		//设置输入的参数类型，
		c.Argv[i] = core.TryObjectEncoding(c.Argv[i])

		//如果列表不存在则创建
		if lobj == nil {
			lobj = core.CreateLinkedListObject()
			//添加到数据库中
			core.DbAdd(c.Db, c.Argv[1], lobj)
		}

		//将值push到列表
		listTypePush(c, lobj, c.Argv[i], where)
		pushed++
	}

	core.AddReplyStatus(c, "(length) " + strconv.Itoa(waiting + lobj.Ptr.(*core.LinkedList).Len))

	if pushed > 0 {
		//SignalModifiedKey(c)
		//notifyKeyspaceEvent()
	}
	s.Dirty += pushed
}



func LpushCommand(c *core.YedisClients, s *core.YedisServer) {
	PushGenericCommand(c, s, core.LIST_HEAD)
}


func RpushCommand(c *core.YedisClients, s *core.YedisServer) {
	PushGenericCommand(c, s, core.LIST_TAIL)
}




func listTypePush(c *core.YedisClients, subject *core.YedisObject, value *core.YedisObject, where int) {
	if subject.Encoding == core.OBJ_ENCODING_LINKEDLIST {
		if where == core.LIST_HEAD {
			core.ListAddNodeHead(subject.Ptr.(*core.LinkedList), value)
		}else {
			core.ListAddNodeTail(subject.Ptr.(*core.LinkedList), value)
		}
	}else {
		core.AddReplyStatus(c, "Unknown list encoding")
	}
}

//如果客户端因为等待key被push阻塞，那么将key放进 server.ready_keys 列表里面
func SignalListAsReady(c *core.YedisClients, s *core.YedisServer, key *core.YedisObject) {
	rl := new(core.ReadyList)

	//判断有没有客户端被这个键阻塞
	if core.LookupKey(c.Db.BlockingKeys, key) == nil {
		return
	}

	//被添加到了ready_keys中也直接返回
	if core.LookupKey(c.Db.ReadyKeys, key) != nil {
		return
	}

	//创建readyList保存键和数据库
	rl.Key = key
	rl.Db = c.Db

	//TODO 减少key的引用并添加到server.ReadyKeys中
	core.ListAddNodeTail(s.ReadyKeys, core.CreateObject(core.REDIS_LIST, core.OBJ_ENCODING_LINKEDLIST, rl))

	//将key添加到c.Db.ReadyKeys中，防止重复添加
	c.Db.ReadyKeys[key] = nil
}