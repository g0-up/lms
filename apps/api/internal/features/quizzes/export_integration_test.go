//go:build integration

package quizzes

// AssertGolden cho test dựng trên seed (package quizzes_test, vì seed import quizzes) dùng chung golden và cờ -update.
var AssertGolden = assertGolden
