package kademlia


import (
	"fmt"
    "container/list"
    "net/rpc"
    "strconv"
    "log"
)

type KBucket struct{
	list.List 
}

func NewKBucket() *KBucket{
	newbucket := &KBucket{}
	newbucket.Init()
	return newbucket
}

//search a node with ID in a bucket
func (l *KBucket) findContact(nodeId ID) (*list.Element,error) {
	fmt.Println("find the node:" + nodeId.AsString())
	for e := l.Front(); e != nil; e = e.Next(){
		c := e.Value.(Contact)
		if nodeId.Equals(c.NodeID){
			return e, nil
		}
	}
	return nil, &NotFoundError{nodeId, "not found in KBucket"}
}

func (l *KBucket) checkFull() bool{
	return l.Len() >= K
}

func (l *KBucket) copyLine() []Contact{
	var nodes []Contact
	for e := l.Front(); e != nil; e = e.Next(){
		c := e.Value.(Contact)
		nodes = append(nodes,c)
	}
	return nodes
}

func (l *KBucket) update(curNode Contact) {
	//condition 1: if the contact exists
	ele,_ := l.findContact(curNode.NodeID)
	if ele != nil{
		l.MoveToBack(ele)
		fmt.Println("update condition 1 successfully")
	}else{
		//condition 2: contact not exist, this KBucket is not full
		ok := l.checkFull() 
		if !ok{
			l.PushBack(curNode)
			fmt.Println("update condition 2 successfully")
		}else{
		//condition 3: contact not exist, this KBucket is full
			least := l.Front()
			leastContact := least.Value.(Contact)//cast the value of list element to Contact
			leastAddress := leastContact.Host.String() + ":" + strconv.Itoa(int(leastContact.Port))
			//go func(){
				//_, ok := curKad.doPing(leastContact.Host, leastContact.Port, false)
				client, err := rpc.DialHTTP("tcp",leastAddress)
				if err != nil{
					log.Fatal("DialHTTP: ", err)
				}
				ping := new(PingMessage)
				ping.MsgID = NewRandomID()
				var pong PongMessage
				err = client.Call("KademliaCore.Ping",ping, &pong)
				if err != nil{
					log.Fatal("Call: ", err)
				}
				if pong.MsgID != ping.MsgID{
					//ping failed, remove the leastContact, add new to tail
					l.Remove(least)
					l.PushBack(curNode)
					//pingResponseChannel <- pingResponse{curkad, leastContact, curNode, false} 
				}else{
					//ping succesfully, move least Contact to tail
					l.MoveToBack(least)
					//pingResponseChannel <- pingResponse{curKad, leastContact, curNode, true}
				}
				fmt.Println("update condition 3 successfully")
			//}()

		}
	}
			
}


 

