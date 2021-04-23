package lugo

type waitPool struct {
	freeList *waitNode
}

func (p *waitPool) init() {
	p.freeList = &waitNode{}
}

func (p *waitPool) put(v chan *RpcResponse) {
	p.freeList.next = &waitNode{
		data: v,
		next: p.freeList.next,
	}
}

func (p *waitPool) get() chan *RpcResponse {
	head := p.freeList.next
	if head != nil {
		p.freeList.next = head.next
		head.next = nil
		return head.data
	} else {
		// 防止写端先写入,读端还未进入读取状态, 设置缓存大小为1
		return make(chan *RpcResponse, 1)
	}
}

func (p *waitPool) size() int {
	sz := 0
	cur := p.freeList.next
	for cur != nil {
		cur = cur.next
		sz++
	}
	return sz
}
