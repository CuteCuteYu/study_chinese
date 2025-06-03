package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

// 全局Redis客户端
var rdb *redis.Client

func main() {
	r := gin.Default()
	r.LoadHTMLGlob("templates/*")
	r.Static("/static", "./templates")

	// 初始化Redis连接
	rdb = redis.NewClient(&redis.Options{
		Addr:     "study_chinese_redis_1:6379",
		Password: "", // 无密码
		DB:       0,  // 使用默认DB
	})

	// 路由配置
	r.GET("/", homeHandler)
	r.GET("/login", loginHandler)
	r.POST("/login", loginHandler)
	r.GET("/register", registerHandler)
	r.POST("/register", registerHandler)
	r.GET("/course/get", courseDetailHandler)
	r.GET("/admin", adminHandler)
	r.GET("/admin/course/add", addCourseHandler)
	r.POST("/admin/course/add", addCoursePostHandler)
	r.POST("/admin/course/delete", deleteCourseHandler)

	// 启动服务器
	fmt.Println("Server starting on port 8080...")
	r.Run(":8080")
}

func homeHandler(c *gin.Context) {
	// 检查Redis连接
	_, err := rdb.Ping(c.Request.Context()).Result()
	if err != nil {
		c.String(http.StatusInternalServerError, "Redis连接失败")
		return
	}

	// 获取用户名参数
	username := c.Query("username")

	// 如果URL中没有用户名参数，检查session
	if username == "" {
		sessionToken, err := c.Cookie("session_token")
		if err != nil {
			c.Redirect(http.StatusFound, "/login")
			return
		}

		// 从Redis获取用户名
		username, err = rdb.Get(c.Request.Context(), "session:"+sessionToken).Result()
		if err != nil {
			c.Redirect(http.StatusFound, "/login")
			return
		}
	}

	// 检查是否是登出请求
	if c.Query("action") == "logout" {
		sessionToken, err := c.Cookie("session_token")
		if err == nil {
			// 从Redis删除session
			rdb.Del(c.Request.Context(), "session:"+sessionToken)
			// 清除cookie
			c.SetCookie("session_token", "", -1, "/", "", false, true)
		}
		c.Redirect(http.StatusFound, "/login")
		return
	}

	// 从Redis获取课程列表
	courses, err := rdb.Keys(c.Request.Context(), "course:*").Result()
	if err != nil {
		c.String(http.StatusInternalServerError, "获取课程列表失败")
		return
	}

	var courseList []gin.H
	for _, courseKey := range courses {
		courseData, err := rdb.HGetAll(c.Request.Context(), courseKey).Result()
		if err != nil {
			continue
		}
		courseList = append(courseList, gin.H{
			"Title":       courseData["title"],
			"Description": courseData["description"],
			"Key":         courseKey,
		})
	}

	// 渲染主页模板
	data := gin.H{"title": "中文学习平台", "courses": courseList}
	if username != "" {
		data["user"] = username
	}
	c.HTML(http.StatusOK, "index.html", data)
}

func loginHandler(c *gin.Context) {
	// 如果是GET请求，显示登录页面
	if c.Request.Method == "GET" {
		flashError, _ := c.Cookie("flash_error")
		c.SetCookie("flash_error", "", -1, "/", "", false, true)
		c.HTML(http.StatusOK, "login.html", gin.H{
			"title":       "用户登录",
			"flash_error": flashError,
		})
		return
	}

	// POST请求处理登录逻辑
	username := c.PostForm("username")
	password := c.PostForm("password")

	// 从Redis验证用户凭据
	storedPass, err := rdb.Get(c.Request.Context(), "user:"+username).Result()
	if err == redis.Nil {
		c.SetCookie("flash_error", "用户不存在", 10, "/", "", false, true)
		c.Redirect(http.StatusFound, "/login")
		return
	} else if err != nil {
		c.SetCookie("flash_error", "服务器错误，请稍后再试", 10, "/", "", false, true)
		c.Redirect(http.StatusFound, "/login")
		return
	}

	if storedPass != password {
		c.SetCookie("flash_error", "密码错误", 10, "/", "", false, true)
		c.Redirect(http.StatusFound, "/login")
		return
	}

	// 创建会话token并存入Redis
	sessionToken := uuid.New().String()
	rdb.Set(c.Request.Context(), "session:"+sessionToken, username, 24*time.Hour)
	c.SetCookie("session_token", sessionToken, 3600, "/", "", false, true)
	c.Redirect(http.StatusFound, "/?username="+username)
}

func courseDetailHandler(c *gin.Context) {
	courseKey := c.Query("key")
	if courseKey == "" {
		c.Redirect(http.StatusFound, "/")
		return
	}

	courseData, err := rdb.HGetAll(c.Request.Context(), courseKey).Result()
	if err != nil {
		c.Redirect(http.StatusFound, "/")
		return
	}

	c.HTML(http.StatusOK, "coursecontent.html", gin.H{
		"Title":       courseData["title"],
		"Description": courseData["description"],
		"Content":     courseData["content"],
	})
}

func adminHandler(c *gin.Context) {
	// 检查是否是admin用户
	username := getUsernameFromContext(c)
	if username != "admin" {
		c.Redirect(http.StatusFound, "/")
		return
	}

	// 从Redis获取课程列表
	courses, err := rdb.Keys(c.Request.Context(), "course:*").Result()
	if err != nil {
		c.String(http.StatusInternalServerError, "获取课程列表失败")
		return
	}

	var courseList []gin.H
	for _, courseKey := range courses {
		courseData, err := rdb.HGetAll(c.Request.Context(), courseKey).Result()
		if err != nil {
			continue
		}
		courseList = append(courseList, gin.H{
			"Title":       courseData["title"],
			"Description": courseData["description"],
			"Key":         courseKey,
		})
	}

	c.HTML(http.StatusOK, "admin.html", gin.H{
		"courses": courseList,
	})
}

func addCourseHandler(c *gin.Context) {
	// 检查是否是admin用户
	username := getUsernameFromContext(c)
	if username != "admin" {
		c.Redirect(http.StatusFound, "/")
		return
	}

	c.HTML(http.StatusOK, "addcourse.html", gin.H{
		"title": "添加课程",
	})
}

func addCoursePostHandler(c *gin.Context) {
	// 检查是否是admin用户
	username := getUsernameFromContext(c)
	if username != "admin" {
		c.Redirect(http.StatusFound, "/")
		return
	}

	title := c.PostForm("title")
	description := c.PostForm("description")
	content := c.PostForm("content")

	if title == "" || description == "" || content == "" {
		c.SetCookie("flash_error", "所有字段都必须填写", 10, "/", "", false, true)
		c.Redirect(http.StatusFound, "/admin/addcourse")
		return
	}

	// 生成课程ID
	courseID := uuid.New().String()
	err := rdb.HSet(c.Request.Context(), "course:"+courseID,
		"title", title,
		"description", description,
		"content", content,
	).Err()

	if err != nil {
		c.SetCookie("flash_error", "保存课程失败", 10, "/", "", false, true)
		c.Redirect(http.StatusFound, "/admin/addcourse")
		return
	}

	c.Redirect(http.StatusFound, "/admin")
}

func deleteCourseHandler(c *gin.Context) {
	// 检查是否是admin用户
	username := getUsernameFromContext(c)
	if username != "admin" {
		c.Redirect(http.StatusFound, "/")
		return
	}

	courseKey := c.PostForm("courseKey")
	if courseKey == "" {
		c.SetCookie("flash_error", "无效的课程ID", 10, "/", "", false, true)
		c.Redirect(http.StatusFound, "/admin")
		return
	}

	// 从Redis删除课程
	err := rdb.Del(c.Request.Context(), courseKey).Err()
	if err != nil {
		c.SetCookie("flash_error", "删除课程失败", 10, "/", "", false, true)
		c.Redirect(http.StatusFound, "/admin")
		return
	}

	c.Redirect(http.StatusFound, "/admin")
}

func getUsernameFromContext(c *gin.Context) string {
	username := c.Query("username")
	if username == "" {
		sessionToken, err := c.Cookie("session_token")
		if err != nil {
			return ""
		}
		username, _ = rdb.Get(c.Request.Context(), "session:"+sessionToken).Result()
	}
	return username
}

func registerHandler(c *gin.Context) {
	if c.Request.Method == "GET" {
		flashError, _ := c.Cookie("flash_error")
		c.SetCookie("flash_error", "", -1, "/", "", false, true)
		c.HTML(http.StatusOK, "register.html", gin.H{
			"title":       "注册",
			"flash_error": flashError,
		})
		return
	}

	username := c.PostForm("username")
	password := c.PostForm("password")

	// 检查用户名是否已存在
	exists, err := rdb.Exists(c.Request.Context(), "user:"+username).Result()
	if err != nil {
		c.SetCookie("flash_error", "服务器错误，请稍后再试", 10, "/", "", false, true)
		c.Redirect(http.StatusFound, "/register")
		return
	}
	if exists == 1 {
		c.SetCookie("flash_error", "用户名已存在", 10, "/", "", false, true)
		c.Redirect(http.StatusFound, "/register")
		return
	}

	// 存储用户信息
	err = rdb.Set(c.Request.Context(), "user:"+username, password, 0).Err()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "注册失败"})
		return
	}

	// 自动登录
	sessionToken := uuid.New().String()
	rdb.Set(c.Request.Context(), "session:"+sessionToken, username, 24*time.Hour)
	c.SetCookie("session_token", sessionToken, 3600, "/", "", false, true)
	c.Redirect(http.StatusFound, "/")
}
