package kvm

type domainXML struct {
	Name   string `xml:"name"`
	UUID   string `xml:"uuid"`
	Memory struct {
		Value int64 `xml:",chardata"`
	} `xml:"memory"`
	CurrentMemory struct {
		Value int64 `xml:",chardata"`
	} `xml:"currentMemory"`
	VCPU struct {
		Current int `xml:"current,attr"`
		Value   int `xml:",chardata"`
	} `xml:"vcpu"`
	Description string `xml:"description"`
	OS          struct {
		Type struct {
			Value   string `xml:",chardata"`
			Machine string `xml:"machine,attr"`
			Arch    string `xml:"arch,attr"`
		} `xml:"type"`
	} `xml:"os"`
	Devices struct {
		Disks      []domainDiskXML `xml:"disk"`
		Interfaces []struct {
			Type string `xml:"type,attr"`
			MAC  struct {
				Address string `xml:"address,attr"`
			} `xml:"mac"`
			Source struct {
				Network string `xml:"network,attr"`
				Bridge  string `xml:"bridge,attr"`
				Dev     string `xml:"dev,attr"`
			} `xml:"source"`
			Target struct {
				Dev string `xml:"dev,attr"`
			} `xml:"target"`
			Model struct {
				Type string `xml:"type,attr"`
			} `xml:"model"`
		} `xml:"interface"`
		Graphics []struct {
			Type   string `xml:"type,attr"`
			Port   string `xml:"port,attr"`
			Listen string `xml:"listen,attr"`
			Passwd string `xml:"passwd,attr"`
		} `xml:"graphics"`
	} `xml:"devices"`
	Metadata struct {
		OSInfo struct {
			Name    string `xml:"name"`
			Version string `xml:"version"`
			ID      string `xml:"id"`
		} `xml:"osinfo"`
	} `xml:"metadata"`
}

type domainDiskXML struct {
	Type   string `xml:"type,attr"`
	Device string `xml:"device,attr"`
	Target struct {
		Dev string `xml:"dev,attr"`
		Bus string `xml:"bus,attr"`
	} `xml:"target"`
	Source       domainDiskSourceXML `xml:"source"`
	BackingStore *domainBackingXML   `xml:"backingStore"`
}

type domainDiskSourceXML struct {
	File string `xml:"file,attr"`
	Dev  string `xml:"dev,attr"`
}

type domainBackingXML struct {
	Source       domainDiskSourceXML `xml:"source"`
	BackingStore *domainBackingXML   `xml:"backingStore"`
}
