package nacos

type configItem struct {
	dataID  string
	group   string
	content *string
}

type configPage struct {
	total float64
	page  float64
	pages float64
	items []configItem
}

type namingInst struct {
	ip          string
	port        float64
	weight      *float64
	healthy     *bool
	enabled     *bool
	ephemeral   *bool
	clusterName *string
	serviceName *string
	metadata    *map[string]string
}

type itemView struct {
	DataID  string  `json:"dataId"`
	Group   string  `json:"group"`
	Content *string `json:"content,omitempty"`
}

type pageView struct {
	TotalCount     float64    `json:"totalCount"`
	PageNumber     float64    `json:"pageNumber"`
	PagesAvailable float64    `json:"pagesAvailable"`
	PageItems      []itemView `json:"pageItems"`
}

type gotView struct {
	DataID  string `json:"dataId"`
	Group   string `json:"group"`
	Content string `json:"content"`
}

type okView struct {
	Success bool `json:"success"`
}

type instView struct {
	IP          string             `json:"ip"`
	Port        float64            `json:"port"`
	Weight      *float64           `json:"weight,omitempty"`
	Healthy     *bool              `json:"healthy,omitempty"`
	Enabled     *bool              `json:"enabled,omitempty"`
	Ephemeral   *bool              `json:"ephemeral,omitempty"`
	ClusterName *string            `json:"clusterName,omitempty"`
	ServiceName *string            `json:"serviceName,omitempty"`
	Metadata    *map[string]string `json:"metadata,omitempty"`
}

func (p configPage) view() pageView {
	items := make([]itemView, 0, len(p.items))
	for _, item := range p.items {
		items = append(items, itemView{DataID: item.dataID, Group: item.group, Content: item.content})
	}
	return pageView{
		TotalCount:     p.total,
		PageNumber:     p.page,
		PagesAvailable: p.pages,
		PageItems:      items,
	}
}

func (n namingInst) view() instView {
	return instView{
		IP:          n.ip,
		Port:        n.port,
		Weight:      n.weight,
		Healthy:     n.healthy,
		Enabled:     n.enabled,
		Ephemeral:   n.ephemeral,
		ClusterName: n.clusterName,
		ServiceName: n.serviceName,
		Metadata:    n.metadata,
	}
}
