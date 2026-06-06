package kvm

import (
	"encoding/xml"
	"strconv"
	"strings"
)

const defaultMemoryStatsPeriodSeconds = 5

func (p *VirshProvider) configureDefaultMemoryStats(vmName string) error {
	return p.setMemoryStatsPeriod(vmName, defaultMemoryStatsPeriodSeconds, false)
}

func (p *VirshProvider) updateMemoryStatsPeriod(vmName string, period int, running bool) error {
	return p.setMemoryStatsPeriod(vmName, period, running)
}

func (p *VirshProvider) setMemoryStatsPeriod(vmName string, period int, running bool) error {
	args := []string{"--connect", p.libvirtURI, "dommemstat", vmName, "--period", strconv.Itoa(period)}
	if running {
		args = append(args, "--live", "--config")
	} else {
		args = append(args, "--config")
	}
	_, err := p.output("virsh", args...)
	return err
}

func memoryStatsPeriodFromXML(input string) int {
	decoder := xml.NewDecoder(strings.NewReader(input))
	inMemballoon := false
	for {
		token, err := decoder.Token()
		if err != nil {
			return 0
		}
		switch item := token.(type) {
		case xml.StartElement:
			if item.Name.Local == "memballoon" {
				inMemballoon = true
				continue
			}
			if inMemballoon && item.Name.Local == "stats" {
				period, _ := strconv.Atoi(strings.TrimSpace(attrValue(item.Attr, "period")))
				if period > 0 {
					return period
				}
			}
		case xml.EndElement:
			if item.Name.Local == "memballoon" {
				inMemballoon = false
			}
		}
	}
}
