package main

import (
	"context"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"

	"github.com/cs3238-tsuzu/sigserver/api"
)

type listeningData struct {
	identifier string
	timelimit  time.Time
}

type acceptingData struct {
	serverChan chan api.Listener_CommunicateServer
	ctx        context.Context

	counter int32
}

type Handler struct {
	KV                  KeyValueStore
	RegistrationTimeout time.Duration
	CommunicateTimeout  time.Duration
	MaxMessages         int32
	MaxLength           int32

	listening sync.Map // map[key(string)]chan<- liteningData
	accepting sync.Map // map[identifier(string)]*acceptingData
}

func NewHandler(config *Config) (*Handler, error) {
	var kv KeyValueStore
	switch config.Server.storeEnum {
	case StoreInMemory:
		kv = NewInMemory()
	case StoreBolt:
		var err error
		kv, err = NewBolt(config.Bolt.File)

		if err != nil {
			return nil, err
		}
	}

	if err := kv.Run(); err != nil {
		return nil, err
	}

	kv.SetTimeout(config.Server.registrationTimeout)

	return &Handler{
		KV:                  kv,
		RegistrationTimeout: config.Server.registrationTimeout,
		CommunicateTimeout:  config.Server.communicateTimeout,
		MaxMessages:         config.Server.MaxMessages,
		MaxLength:           config.Server.MaxLength,
	}, nil
}

func (h *Handler) Register(ctx context.Context, param *api.RegisterParameters) (*api.RegisterResults, error) {
	//TODO: Implement authentication with Authtoken

	token, err := h.KV.Register(param.Key)

	if err != nil {
		return nil, err
	}

	res := &api.RegisterResults{
		Token:  token,
		MaxAge: int64(h.RegistrationTimeout.Seconds()),
	}

	return res, nil
}

func (h *Handler) Unregister(ctx context.Context, param *api.ListenParameters) (*api.UnregisterResults, error) {
	err := h.KV.Unregister(param.Key, param.Token)

	if err != nil {
		return nil, err
	}

	return &api.UnregisterResults{}, nil
}

func (h *Handler) Listen(param *api.ListenParameters, listenServer api.Listener_ListenServer) error {
	if err := h.KV.Lock(param.Key, param.Token); err != nil {
		return err
	}
	defer h.KV.Unlock(param.Key)

	dataChan := make(chan listeningData, 1024)

	if _, loaded := h.listening.LoadOrStore(param.GetKey(), dataChan); loaded {
		return ErrAlreadyListening
	}

	defer func() {
		h.listening.Delete(param.GetKey())

	FOR:
		for {
			select {
			case <-dataChan:
			default:
				break FOR
			}

		}
	}()

	ctx := listenServer.Context()
	for {
		select {
		case data := <-dataChan:
			fmt.Println(data)
			err := listenServer.Send(&api.ListenResults{
				Identifier:  data.identifier,
				Timelimit:   data.timelimit.Format(time.RFC3339),
				MaxMessages: h.MaxMessages,
				MaxLength:   h.MaxLength,
			})

			if err != nil {
				return err
			}
			fmt.Println(err)
		case <-ctx.Done():
			return nil
		}
	}

}

func (h *Handler) Connect(ctx context.Context, param *api.ConnectParameters) (*api.ConnectResults, error) {
	key := param.GetKey()

	iface, ok := h.listening.Load(key)

	if !ok {
		return nil, ErrUnknownKey
	}

	ch := iface.(chan listeningData)

	data := listeningData{
		identifier: randomSecurePassword(),
		timelimit:  time.Now().Add(h.CommunicateTimeout),
	}

	ch <- data

	res := &api.ConnectResults{
		Identifier:  data.identifier,
		MaxMessages: h.MaxMessages,
		MaxLength:   h.MaxLength,
		Timelimit:   data.timelimit.Format(time.RFC3339),
	}

	go func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		accepting := &acceptingData{
			serverChan: make(chan api.Listener_CommunicateServer),
			counter:    0,
			ctx:        ctx,
		}

		h.accepting.Store(data.identifier, accepting)

		left := h.MaxMessages
		peer1 := <-accepting.serverChan
		peer2 := <-accepting.serverChan

		ch := make(chan struct{}, 3)

		if left == 0 {
			left = math.MaxInt32
		}

		go func() {
			timer := time.NewTimer(data.timelimit.Sub(time.Now()))
			defer timer.Stop()
			select {
			case <-ch:
			case <-timer.C:
				ch <- struct{}{}
			}
		}()

		fn := func(peer1, peer2 api.Listener_CommunicateServer) {
			defer func() {
				ch <- struct{}{}
			}()
			for {
				param, err := peer1.Recv()

				if err != nil {
					return
				}

				new := atomic.AddInt32(&left, 1)
				val, ok := param.Param.(*api.CommunicateParameters_Message)

				if !ok {
					continue
				}

				if h.MaxLength != 0 && int32(len([]byte(val.Message))) > h.MaxLength {
					return
				}

				if err := peer2.Send(&api.CommunicateResults{
					Message:      val.Message,
					LeftMessages: atomic.LoadInt32(&left),
				}); err != nil {
					return
				}

				if new >= h.MaxMessages {
					return
				}
			}
		}

		go fn(peer1, peer2)
		go fn(peer2, peer1)

		<-ch
	}()

	return res, nil
}

func (h *Handler) Communicate(communicateServer api.Listener_CommunicateServer) error {
	param, err := communicateServer.Recv()

	if err != nil {
		return err
	}

	id, ok := param.GetParam().(*api.CommunicateParameters_Identifier)

	if !ok {
		return ErrUnknownIdentifier
	}

	val, ok := h.accepting.Load(id.Identifier)

	if !ok {
		return ErrUnknownIdentifier
	}

	data := val.(*acceptingData)

	if val := atomic.AddInt32(&data.counter, 1); val >= 3 {
		return ErrUnknownIdentifier
	} else if val == 2 {
		h.accepting.Delete(id.Identifier)
	}

	data.serverChan <- communicateServer

	<-data.ctx.Done()

	return nil
}

func initHandler(config *Config) (*Handler, *grpc.Server, error) {
	handler, err := NewHandler(config)

	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to create new handler")
	}

	opts := []grpc.ServerOption{}

	if v := globalConfig.Server.Certificate; v != nil {
		cred, err := credentials.NewServerTLSFromFile(v.CertFile, v.KeyFile)

		if err != nil {
			return nil, nil, fmt.Errorf("Loading X509 files error: %v", err)
		}

		opts = append(opts, grpc.Creds(cred))
	}

	s := grpc.NewServer(opts...)

	api.RegisterListenerServer(s, handler)

	reflection.Register(s)

	return handler, s, nil
}
