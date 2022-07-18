package handlers

import (
	"errors"
	"io/ioutil"
	"main/internal"
	"os"
	"text/template"
)

type TfSvs struct {
	Name string
	Dir  string
	Cfg  TfCfg

	tfTemplateFile string
	tfBin          string
}

type TfCfg struct {
	GcpProjectId string
	Stackname    string
	Region       string
	DB_USER      string
	DB_PASS      string
	Env          string
}

func NewTfSvs(name string, clusterCfg clusterCfg) (*TfSvs, error) {

	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	tfSvs := &TfSvs{
		Name: name,
		Dir:  wd + "/_files/tf/" + name,
		Cfg: TfCfg{
			GcpProjectId: internal.Cfg.Gcps.ProjectId,
			Stackname:    clusterCfg.Stackname,
			Region:       clusterCfg.Region,
			DB_USER:      clusterCfg.DB_USER,
			DB_PASS:      clusterCfg.DB_PASS,
			Env:          clusterCfg.Env,
		},

		tfTemplateFile: wd + "/_files/tf/gcp.tf.gotemplate",
		tfBin:          "terraform",
	}

	return tfSvs, nil
}

func (tf *TfSvs) Run(flags ...string) (string, []string, error) {
	wd, _ := os.Getwd()
	// render the template.tf with cfg.Stackname into a Stackname named folder so that
	// 1. we can run terraform from that folder
	// 2. terraform will use a Stackname named folder in it's remote backend
	if _, err := os.Stat(tf.tfTemplateFile); errors.Is(err, os.ErrNotExist) {
		return "", nil, err
	}

	tfdir := wd + "/_files/tf/" + tf.Cfg.Stackname
	os.Mkdir(tfdir, os.ModePerm)

	tfFile := tfdir + "/rendered.tf"
	t, err := template.ParseFiles(tf.tfTemplateFile)
	if err != nil {
		return "", nil, err
	}
	f, _ := os.Create(tfFile)
	defer f.Close()

	t.Execute(f, struct{ ProjectId, Stackname, Region, DbUser, DbPass, Env string }{
		ProjectId: tf.Cfg.GcpProjectId,
		Stackname: tf.Cfg.Stackname,
		Region:    tf.Cfg.Region,
		DbUser:    tf.Cfg.DB_USER,
		DbPass:    tf.Cfg.DB_PASS,
		Env:       tf.Cfg.Env,
	})
	tfBytes, _ := ioutil.ReadFile(tfFile)
	tfFileStr := string(tfBytes)

	err, tf_out_init := internal.RunCmd_sync(tf.tfBin, "-chdir="+tfdir, "init")
	if err != nil {
		return tfFileStr, nil, err
	}

	args := []string{"-chdir=" + tfdir}
	for _, flag := range flags {
		args = append(args, flag)
	}
	err, tf_out_verb := internal.RunCmd_sync(tf.tfBin, args...)
	if err != nil {
		return "", nil, err
	}
	return tfFileStr, append(tf_out_init, tf_out_verb...), nil
}
