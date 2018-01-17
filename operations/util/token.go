package util

import (
	"golang.org/x/net/context"
	authnv1 "k8s.io/api/authentication/v1"
	"k8s.io/client-go/kubernetes"
)

// This exists because one is not supposed to use any built in types for keys in context.WithValue()
type authContext string

var (
	tokenAuthInfo = authContext("tokenInfo")
	clientSetInfo = authContext("clientSet")
)

func GetClientset(ctx context.Context) *kubernetes.Clientset {
	clientset := ctx.Value(clientSetInfo)
	cs, ok := clientset.(*kubernetes.Clientset)
	if !ok {
		// TODO panic is shitty
		panic("Tried to GetClientset but didn't PutClientset")
	}
	return cs
}

func PutClientset(ctx context.Context, clientset *kubernetes.Clientset) context.Context {
	return context.WithValue(ctx, clientSetInfo, clientset)
}

func GetToken(ctx context.Context) *authnv1.TokenReview {
	tokenInfo := ctx.Value(tokenAuthInfo)
	ti, ok := tokenInfo.(*authnv1.TokenReview)
	if !ok {
		// TODO panic is shitty
		panic("Tried to GetToken but didn't PutToken")
	}
	return ti
}

func PutToken(ctx context.Context, tokenInfo *authnv1.TokenReview) context.Context {
	// save the TokenReview api object to the context for later use
	return context.WithValue(ctx, tokenAuthInfo, tokenInfo)
}
