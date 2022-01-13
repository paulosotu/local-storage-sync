package models

type Node struct {
	name   string
	ip     string
	status string
}

func NewNode(name, ip, status string) *Node {
	return &Node{
		name:   name,
		ip:     ip,
		status: status,
	}
}

func (n *Node) GetName() string {
	return n.name
}

func (n *Node) GetIP() string {
	return n.ip
}

func (n *Node) GetStatus() string {
	return n.status
}
