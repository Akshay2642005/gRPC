package main

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"

	pb "github.com/Akshay2642005/gRPC/proto/tictactoe"
	"google.golang.org/grpc"
)

type Room struct {
	ID          string
	Players     map[string]string // Player ID -> Symbol (e.g., "X" or "O")
	CurrentTurn string            // Player ID of the current turn
	Board       [][]string        // 3x3 board
	IsGameOver  bool
	Winner      string
	Mutex       sync.Mutex
}

type TicTacToeServer struct {
	pb.UnimplementedTicTacToeServiceServer
	Rooms map[string]*Room // Room ID -> Room
	Mutex sync.Mutex
}

// Utility function to create a random room ID
func generateRoomID() string {
	rand.Seed(time.Now().UnixNano())
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

// Initialize an empty board
func createEmptyBoard() [][]string {
	return [][]string{
		{"", "", ""},
		{"", "", ""},
		{"", "", ""},
	}
}

// CreateRoom allows a player to create a new room
func (s *TicTacToeServer) CreateRoom(ctx context.Context, req *pb.CreateRoomRequest) (*pb.CreateRoomResponse, error) {
	roomID := generateRoomID()
	room := &Room{
		ID:          roomID,
		Players:     map[string]string{req.PlayerName: "X"}, // First player gets "X"
		CurrentTurn: req.PlayerName,
		Board:       createEmptyBoard(),
	}
	s.Mutex.Lock()
	s.Rooms[roomID] = room
	s.Mutex.Unlock()

	return &pb.CreateRoomResponse{RoomId: roomID}, nil
}

// JoinRoom allows another player to join an existing room
func (s *TicTacToeServer) JoinRoom(ctx context.Context, req *pb.JoinRoomRequest) (*pb.JoinRoomResponse, error) {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()

	room, exists := s.Rooms[req.RoomId]
	if !exists {
		return nil, fmt.Errorf("room not found")
	}

	if len(room.Players) >= 2 {
		return nil, fmt.Errorf("room is full")
	}

	// Assign the new player the "O" symbol
	room.Players[req.PlayerName] = "O"

	return &pb.JoinRoomResponse{Message: "Successfully joined the room!"}, nil
}

// PlaceMove handles player moves and updates the game state
func (s *TicTacToeServer) PlaceMove(ctx context.Context, req *pb.PlaceMoveRequest) (*pb.GameState, error) {
	s.Mutex.Lock()
	room, exists := s.Rooms[req.RoomId]
	if !exists {
		s.Mutex.Unlock()
		return nil, fmt.Errorf("room not found")
	}
	room.Mutex.Lock()
	defer s.Mutex.Unlock()
	defer s.Mutex.Unlock()

	// Validate move
	if room.Board[req.Row][req.Col] != "" {
		return nil, fmt.Errorf("cell already occupied")
	}
	if room.IsGameOver {
		return nil, fmt.Errorf("game is already over")
	}

	// Place the move
	symbol := room.Players[req.PlayerName]
	room.Board[req.Row][req.Col] = symbol

	// Check for a win
	if checkWin(room.Board, symbol) {
		room.IsGameOver = true
		room.Winner = req.PlayerName
	}

	// Check for a draw
	if !room.IsGameOver && checkDraw(room.Board) {
		room.IsGameOver = true
		room.Winner = ""
	}

	// Switch turns
	for player := range room.Players {
		if player != req.PlayerName {
			room.CurrentTurn = player
			break
		}
	}

	return &pb.GameState{
		Board:      room.Board,
		IsGameOver: room.IsGameOver,
		Winner:     room.Winner,
	}, nil
}

// StreamGameState streams the game state to players in a room
func (s *TicTacToeServer) StreamGameState(req *pb.RoomRequest, stream pb.TicTacToeService_StreamGameStateServer) error {
	roomID := req.RoomId

	for {
		s.Mutex.Lock()
		room, exists := s.Rooms[roomID]
		s.Mutex.Unlock()
		if !exists {
			return fmt.Errorf("room not found")
		}

		// Send the game state
		err := stream.Send(&pb.GameState{
			Board:      room.Board,
			IsGameOver: room.IsGameOver,
			Winner:     room.Winner,
		})
		if err != nil {
			return err
		}

		time.Sleep(1 * time.Second)
	}
}

// Helper function to check for a win
func checkWin(board [][]string, symbol string) bool {
	for i := 0; i < 3; i++ {
		// Check rows and columns
		if (board[i][0] == symbol && board[i][1] == symbol && board[i][2] == symbol) ||
			(board[0][i] == symbol && board[1][i] == symbol && board[2][i] == symbol) {
			return true
		}
	}

	// Check diagonals
	return (board[0][0] == symbol && board[1][1] == symbol && board[2][2] == symbol) ||
		(board[0][2] == symbol && board[1][1] == symbol && board[2][0] == symbol)
}

// Helper function to check for a draw
func checkDraw(board [][]string) bool {
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			if board[i][j] == "" {
				return false
			}
		}
	}
	return true
}

func main() {
	// Create gRPC server
	server := grpc.NewServer()
	pb.RegisterTicTacToeServiceServer(server, &TicTacToeServer{Rooms: make(map[string]*Room)})

	listener, err := net.Listen("tcp", ":50051")
	if err != nil {
		fmt.Println("Failed to start server:", err)
		return
	}

	fmt.Println("Tic-Tac-Toe server started on :50051")
	if err := server.Serve(listener); err != nil {
		fmt.Println("Server failed:", err)
	}
}
