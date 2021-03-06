package gotsrpc

import (
	"strings"

	"github.com/foomo/gotsrpc/config"
)

func renderPHPRPCServiceClients(service *Service, namespce string, g *code) error {
	g.l(`<?php`)
	g.l(`// Code generated by gotsrpc https://github.com/foomo/gotsrpc DO NOT EDIT.`)
	g.nl()
	g.l(`namespace ` + namespce + `;`)
	g.nl()

	g.l(`class ` + service.Name + `Client`)
	g.l(`{`)
	g.ind(1)

	// Variables
	g.l(`public static $defaultOptions = ['http' => ['method' => 'POST', 'timeout' => 5, 'header'  => 'Content-type: application/json']];`)
	g.nl()
	g.l(`private $options;`)
	g.l(`private $endpoint;`)
	g.nl()

	// Constructor
	g.l(`/**`)
	g.l(` * @param string $endpoint`)
	g.l(` * @param array $options`)
	g.l(` */`)
	g.l(`public function __construct($endpoint, array $options=null)`)
	g.l(`{`)
	g.ind(1)
	g.l(`$this->endpoint = $endpoint;`)
	g.l(`$this->options = (is_null($options)) ? self::$defaultOptions : $options;`)
	g.ind(-1)
	g.l(`}`)
	g.nl()

	// Service methods
	for _, method := range service.Methods {
		params := []string{}

		g.l(`/**`)
		for i, a := range method.Args {
			if i == 0 && a.Value.isHTTPResponseWriter() {
				continue
			}
			if i == 1 && a.Value.isHTTPRequest() {
				continue
			}
			params = append(params, "$"+a.Name)
			g.l(` * @param $` + a.Name)
		}
		g.l(` * @return array`)
		g.l(` */`)
		g.l(`public function ` + lcfirst(method.Name) + `(` + strings.Join(params, ", ") + `)`)
		g.l(`{`)
		g.ind(1)
		g.l(`return $this->call('` + method.Name + `', [` + strings.Join(params, ", ") + `]);`)
		g.ind(-1)
		g.l(`}`)
		g.nl()
	}

	// Protected methods
	g.l(`/**`)
	g.l(` * @param string $method`)
	g.l(` * @param array $request`)
	g.l(` * @return array`)
	g.l(` * @throws \Exception`)
	g.l(` */`)
	g.l(`protected function call($method, array $request)`)
	g.l(`{`)
	g.ind(1)
	g.l(`$options = $this->options;`)
	g.l(`$options['http']['content'] = json_encode($request);`)
	g.l(`if (false === $content = @file_get_contents($this->endpoint . '/' . $method, false, stream_context_create($options))) {`)
	g.ind(1)
	g.l(`$err = error_get_last();`)
	g.l(`throw new \Exception($err['message'], $err['type']);`)
	g.ind(-1)
	g.l(`}`)
	g.l(`return json_decode($content);`)
	g.ind(-1)
	g.l(`}`)
	g.nl()

	g.ind(-1)
	g.l(`}`)
	g.nl()
	return nil
}

func RenderPHPRPCClients(services ServiceList, config *config.Target) (code map[string]string, err error) {
	code = map[string]string{}
	for _, service := range services {
		// Check if we should render this service as ts rcp
		// Note: remove once there's a separate gorcp generator
		if !config.IsPHPRPC(service.Name) {
			continue
		}
		target := config.GetPHPTarget(service.Name)
		g := newCode("	")
		if err = renderPHPRPCServiceClients(service, target.Namespace, g); err != nil {
			return
		}
		code[target.Out+"/"+service.Name+"Client.php"] = g.string()
	}
	return
}
