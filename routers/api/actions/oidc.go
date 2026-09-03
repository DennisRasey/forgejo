// Copyright 2025 The Forgejo Authors. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"forgejo.org/modules/jwtx"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/web"
	web_types "forgejo.org/modules/web/types"
	actions_service "forgejo.org/services/actions"
	"forgejo.org/services/context"
)

type oidcRoutes struct {
	openIDConfiguration openIDConfiguration
	jwks                map[string][]map[string]string
}

type openIDConfiguration struct {
	Issuer                           string   `json:"issuer"`
	JwksURI                          string   `json:"jwks_uri"`
	SubjectTypesSupported            []string `json:"subject_types_supported"`
	ResponseTypesSupported           []string `json:"response_types_supported"`
	ClaimsSupported                  []string `json:"claims_supported"`
	IDTokenSigningAlgValuesSupported []string `json:"id_token_signing_alg_values_supported"`
	ScopesSupported                  []string `json:"scopes_supported"`
}

type oidcContextKeyType struct{}

var oidcContextKey = oidcContextKeyType{}

// jwtSigningKey is the default signing key for JWTs.
var jwtSigningKey jwtx.SigningKey

// jwk is the JWK format of the jwtSigningKey.
var jwk map[string]string

type OIDCContext struct {
	*context.Base
}

func InitOIDC() error {
	var err error
	jwtSigningKey, err = jwtx.InitAsymmetricSigningKey(setting.Actions.IDTokenSigningPrivateKeyFile, string(setting.Actions.IDTokenSigningAlgorithm))
	if err != nil {
		return err
	}

	jwk, err = jwtSigningKey.ToJWK()
	if err != nil {
		return fmt.Errorf("Error getting JWK from default signing key: %v", err)
	}
	jwk["use"] = "sig"

	return nil
}

func init() {
	web.RegisterResponseStatusProvider[*OIDCContext](func(req *http.Request) web_types.ResponseStatusProvider {
		return req.Context().Value(oidcContextKey).(*OIDCContext)
	})
}

func OIDCContexter() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			base, baseCleanUp := context.NewBaseContext(resp, req)
			defer baseCleanUp()

			ctx := &OIDCContext{Base: base}
			ctx.AppendContextValue(oidcContextKey, ctx)

			next.ServeHTTP(ctx.Resp, ctx.Req)
		})
	}
}

func OIDCRoutes(prefix string) *web.Route {
	m := web.NewRoute()

	prefix = strings.TrimPrefix(prefix, "/")

	// Standard claims
	claimsSupported := []string{
		"sub",
		"aud",
		"exp",
		"iat",
		"iss",
		"nbf",
	}

	// Add custom claims by iterating over [actions_service.IDTokenCustomClaims]
	// and inspecting the names of the json struct tags
	rt := reflect.TypeFor[actions_service.IDTokenCustomClaims]()

	for f := range rt.Fields() {
		v := strings.Split(f.Tag.Get("json"), ",")[0]
		if v == "" || v == "-" {
			continue
		}

		claimsSupported = append(claimsSupported, v)
	}

	o := &oidcRoutes{
		openIDConfiguration: openIDConfiguration{
			Issuer:                           setting.AppURL + prefix,
			JwksURI:                          setting.AppURL + prefix + "/.well-known/keys",
			SubjectTypesSupported:            []string{"public"},
			ResponseTypesSupported:           []string{"id_token"},
			ClaimsSupported:                  claimsSupported,
			IDTokenSigningAlgValuesSupported: []string{string(setting.Actions.IDTokenSigningAlgorithm)},
			ScopesSupported:                  []string{"openid"},
		},
		jwks: map[string][]map[string]string{
			"keys": {
				jwk,
			},
		},
	}

	m.Group("", func() {
		m.Get("/keys", o.keys)
		m.Get("/openid-configuration", o.configuration)
	}, OIDCContexter())

	return m
}

func (o *oidcRoutes) configuration(ctx *OIDCContext) {
	ctx.JSON(http.StatusOK, o.openIDConfiguration)
}

func (o *oidcRoutes) keys(ctx *OIDCContext) {
	ctx.JSON(http.StatusOK, o.jwks)
}
