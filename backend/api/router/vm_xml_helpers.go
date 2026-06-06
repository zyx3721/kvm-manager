package router

import (
	"encoding/xml"
	"errors"
	"strings"
)

func domainNameFromXML(input string) (string, error) {
	var doc struct {
		Name string `xml:"name"`
	}
	if err := xml.Unmarshal([]byte(strings.TrimSpace(input)), &doc); err != nil {
		return "", errors.New("虚拟机 XML 格式不正确")
	}
	name := strings.TrimSpace(doc.Name)
	if name == "" {
		return "", errors.New("请在 XML 中配置虚拟机名称")
	}
	return name, nil
}
