package util

import (
	"golang.org/x/net/context"
	authnv1 "k8s.io/api/authentication/v1"
)

// This exists because one is not supposed to use any built in types for keys in context.WithValue()
type authContext string

var (
	tokenAuthInfo = authContext("tokenInfo")
)

func GetToken(ctx context.Context) *authnv1.TokenReview {
	tokenInfo := ctx.Value(tokenAuthInfo)
	ti, ok := tokenInfo.(*authnv1.TokenReview)
	if !ok {
		panic("Tried to get a token but didn't putToken")
	}
	return ti
}

func PutToken(ctx context.Context, tokenInfo *authnv1.TokenReview) context.Context {
	// save the TokenReview api object to the context for later use
	return context.WithValue(ctx, tokenAuthInfo, tokenInfo)
}
