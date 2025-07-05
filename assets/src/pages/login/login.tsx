import { useEffect, useState } from "react"
import { useNavigate } from "react-router-dom"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Card, CardContent, CardFooter, CardHeader, CardTitle } from "@/components/ui/card"
import { Checkbox } from "@/components/ui/checkbox"
import { Loader2 } from "lucide-react"
import { toast } from "sonner"
import axios, { type BaseResponse } from "@utils/axios";
import { AxiosError } from "axios"

export default function Login() {
  const navigate = useNavigate()
  const [rememberMe, setRememberMe] = useState(false)
  const [email, setEmail] = useState("")
  const [password, setPassword] = useState("")
  const [isLoading, setIsLoading] = useState(false)

  const [serverError, setServerError] = useState('');
  const [errors, setErrors] = useState({
    email: "",
    password: ""
  })

  interface LoginResponse {
    token: string;
    refresh_token: string;
    user: {
      name: string;
      email: string;
    }
  }

  const validateForm = () => {
    let valid = true
    const newErrors = {
      email: "",
      password: ""
    }

    if (!email) {
      newErrors.email = "邮箱不能为空"
      valid = false
    } else if (!/\S+@\S+\.\S+/.test(email)) {
      newErrors.email = "邮箱格式不正确"
      valid = false
    }

    if (!password) {
      newErrors.password = "密码不能为空"
      valid = false
    } else if (password.length < 6) {
      newErrors.password = "密码至少6位"
      valid = false
    }

    setErrors(newErrors)
    return valid
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()

    if (!validateForm()) return;
    setServerError('');

    setIsLoading(true)


    try {
      axios.post<BaseResponse<LoginResponse>>("/auth/login", { "email": email, "password": password }).then((res) => {
        setIsLoading(false)
        if (res.data.code !== 200) {
          toast.error("登录失败", {
            description: res.data.error,
          })
          setServerError(res.data.error || "")
          return;
        } else {
          toast.success("登录成功")
          if (rememberMe) {
            localStorage.setItem("rememberedEmail", email)
          } else {
            localStorage.removeItem("rememberedEmail")
          }
          localStorage.setItem("token", res.data.data.token)
          localStorage.setItem("refreshToken", res.data.data.refresh_token)
          localStorage.setItem("name", res.data.data.user.name)
          localStorage.setItem("email", res.data.data.user.email)

          navigate("/sites")

        }
      }).catch((err) => {
        toast.error("登录失败", {
          description: err instanceof AxiosError ? err.response?.data.error : "未知错误",
        })
        setServerError(err instanceof AxiosError ? err.response?.data.error : "未知错误")
      })

    } catch (err) {
      toast.error("登录失败", {
        description: err instanceof AxiosError ? err.response?.data.error : "未知错误",
      })
      setServerError(err instanceof AxiosError ? err.response?.data.error : "未知错误")
    } finally {
      setIsLoading(false)
    }
  }

  useEffect(() => {
    const checkAuthState = async () => {
      try {
        const { data } = await axios.get<BaseResponse<string>>("/auth/state");
        // 如果系统未初始化，跳转到 setup 页面
        if (data.data === 'not_initialized') {
          navigate("/setup");
          return;
        }

        // 原有的 token 检查逻辑
        const token = localStorage.getItem("token");
        if (token) {
          navigate("/sites");
        }
        // 原有的记住我逻辑
        const rememberedEmail = localStorage.getItem("rememberedEmail");
        if (rememberedEmail) {
          setEmail(rememberedEmail);
          setRememberMe(true);
        }
      } catch (err) {
        console.error("检查系统状态失败:", err);
        // 可以添加 toast 提示或静默处理
      }
    };

    checkAuthState();
  }, [])

  return (
    <div className="flex items-center justify-center h-full">
      <Card className="w-full max-w-md items-center justify-center">
        <CardHeader className="w-full">
          <CardTitle className="text-2xl text-center">登录</CardTitle>
        </CardHeader>
        <CardContent className="w-full">

          {serverError && (
            <div className="mb-4 rounded-md bg-destructive/15 p-3 text-sm text-destructive">
              {serverError}
            </div>
          )}
          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="email">邮箱地址</Label>
              <Input
                id="email"
                type="email"
                placeholder="your@email.com"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                className={errors.email ? "border-red-500" : ""}
              />
              {errors.email && <p className="text-sm text-red-500">{errors.email}</p>}
            </div>
            <div className="space-y-2">
              <Label htmlFor="password">密码</Label>
              <Input
                id="password"
                type="password"
                placeholder=""
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                className={errors.password ? "border-red-500" : ""}
              />
              {errors.password && <p className="text-sm text-red-500">{errors.password}</p>}
            </div>
            <div className="flex items-center space-x-2">
              <Checkbox
                id="remember-me"
                checked={rememberMe}
                onCheckedChange={() => setRememberMe(!rememberMe)}
              />
              <Label htmlFor="remember-me" className="text-sm">
                记住账号
              </Label>
            </div>
            <Button type="submit" className="w-full" disabled={isLoading}>
              {isLoading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              {isLoading ? "登录中..." : "登录"}
            </Button>
          </form>
        </CardContent>
        <CardFooter className="flex flex-col space-y-2">
          <Button variant="link" className="w-full text-sm">
            忘记密码？
          </Button>
        </CardFooter>
      </Card>
    </div>
  )
}