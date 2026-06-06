package kvm

import (
	"encoding/xml"
	"strings"
)

func (p *VirshProvider) persistentDomainXML(vmName string) (string, domainXML, error) {
	xmlOut, err := p.output("virsh", "--connect", p.libvirtURI, "dumpxml", vmName)
	if err != nil {
		return "", domainXML{}, err
	}
	doc, err := parseDomainXML(xmlOut)
	return xmlOut, doc, err
}

func (p *VirshProvider) inactiveDomainXML(vmName string) (string, domainXML, error) {
	xmlOut, err := p.output("virsh", "--connect", p.libvirtURI, "dumpxml", vmName, "--inactive")
	if err != nil {
		return "", domainXML{}, err
	}
	doc, err := parseDomainXML(xmlOut)
	return xmlOut, doc, err
}

func (p *VirshProvider) vmSecurityXML(vmName string, fallback string) (string, error) {
	xmlOut, err := p.output("virsh", "--connect", p.libvirtURI, "dumpxml", vmName, "--security-info")
	if err == nil {
		return xmlOut, nil
	}
	if strings.TrimSpace(fallback) != "" {
		return fallback, nil
	}
	return "", err
}

func parseDomainXML(xmlOut string) (domainXML, error) {
	var doc domainXML
	err := xml.Unmarshal([]byte(strings.TrimSpace(xmlOut)), &doc)
	return doc, err
}
