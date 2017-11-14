package main

import (
	"github.com/Sirupsen/logrus"
)

type FlexVol struct{}

func (v *FlexVol) Init() error { return nil }

func (v *FlexVol) Create(options map[string]interface{}) (map[string]interface{}, error) {
	logrus.Infof("%#v", options)
	return map[string]interface{}{}, nil
}

func (v *FlexVol) Delete(options map[string]interface{}) error {
	logrus.Infof("%#v", options)
	return nil
}

func (v *FlexVol) Attach(params map[string]interface{}) (string, error) {
	logrus.Infof("%#v", params)
	return "", nil
}

func (v *FlexVol) Detach(device string) error {
	logrus.Infof("Device: %s", device)
	return nil
}

func (v *FlexVol) Mount(dir string, device string, params map[string]interface{}) error {
	logrus.Infof("Device: %s, dir: %s, params: %#v", device, dir, params)
	return nil
}

func (v *FlexVol) Unmount(dir string) error {
	logrus.Infof("Dir: %s", dir)
	return nil
}
