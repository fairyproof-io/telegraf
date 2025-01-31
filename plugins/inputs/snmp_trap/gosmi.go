package snmp_trap

import (
	"github.com/fairyproof-io/telegraf"
	"github.com/fairyproof-io/telegraf/internal/snmp"
)

type gosmiTranslator struct {
}

func (t *gosmiTranslator) lookup(oid string) (snmp.MibEntry, error) {
	return snmp.TrapLookup(oid)
}

func newGosmiTranslator(paths []string, log telegraf.Logger) (*gosmiTranslator, error) {
	err := snmp.LoadMibsFromPath(paths, log, &snmp.GosmiMibLoader{})
	if err == nil {
		return &gosmiTranslator{}, nil
	}
	return nil, err
}
