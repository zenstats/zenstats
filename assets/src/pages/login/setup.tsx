import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Loader2 } from 'lucide-react';
import axios, { type BaseResponse } from "@utils/axios";

interface SetupFormData {
  email: string;
  password: string;
  name: string;
}

interface SetupResponse {
  message: string;
  user: {
    id: string;
    name: string;
    email: string;
  }
}

export default function Setup() {
  const navigate = useNavigate();
  const [isLoading, setIsLoading] = useState(false);
  const [formData, setFormData] = useState<SetupFormData>({
    name: '',
    email: '',
    password: ''
  });
  const [errors, setErrors] = useState<Record<string, string>>({
    email: '',
    password: '',
    name: ''
  });
  const [serverError, setServerError] = useState('');

  const validateForm = (): boolean => {
    const newErrors: Record<string, string> = {};
    let isValid = true;

    if (!formData.name.trim()) {
      newErrors.name = '管理员名称不能为空';
      isValid = false;
    }

    if (!formData.email) {
      newErrors.email = '邮箱不能为空';
      isValid = false;
    } else if (!/\S+@\S+\.\S+/.test(formData.email)) {
      newErrors.email = '邮箱格式不正确';
      isValid = false;
    }

    if (!formData.password) {
      newErrors.password = '密码不能为空';
      isValid = false;
    } else if (formData.password.length < 6) {
      newErrors.password = '密码至少6位';
      isValid = false;
    }

    setErrors(newErrors);
    return isValid;
  };

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const { name, value } = e.target;
    setFormData(prev => ({ ...prev, [name]: value }));

    // 清除对应字段的错误提示
    if (errors[name]) {
      setErrors(prev => ({ ...prev, [name]: '' }));
    }

    // 清除服务器错误提示
    if (serverError) {
      setServerError('');
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!validateForm()) {
      return;
    }

    setIsLoading(true);
    setServerError('');

    try {
      const response = await axios.post<BaseResponse<SetupResponse>>('/auth/init', {
        name: formData.name,
        email: formData.email,
        password: formData.password
      });

      if (response.data.code === 200) {
        // 初始化成功，重定向到登录页
        navigate('/login', {
          state: { success: '管理员账户创建成功，请登录' }
        });
      } else {
        setServerError(response.data.message || '初始化失败，请重试');
      }
    } catch (error) {
      console.error('Setup error:', error);
      setServerError('服务器错误，请稍后重试');
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="flex h-full items-center justify-center">
      <Card className="w-full max-w-md">
        <CardHeader>
          <CardTitle className="text-2xl">系统初始化</CardTitle>
          <CardDescription>
            创建管理员账户以开始使用系统
          </CardDescription>
        </CardHeader>
        <CardContent>
          {serverError && (
            <div className="mb-4 rounded-md bg-destructive/15 p-3 text-sm text-destructive">
              {serverError}
            </div>
          )}
          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="name">管理员名称</Label>
              <Input
                id="name"
                name="name"
                value={formData.name}
                onChange={handleChange}
                className={errors.name ? 'border-destructive' : ''}
              />
              {errors.name && (
                <p className="text-sm text-destructive">{errors.name}</p>
              )}
            </div>
            <div className="space-y-2">
              <Label htmlFor="email">邮箱地址</Label>
              <Input
                id="email"
                name="email"
                type="email"
                placeholder="your@email.com"
                value={formData.email}
                onChange={handleChange}
                className={errors.email ? 'border-destructive' : ''}
              />
              {errors.email && (
                <p className="text-sm text-destructive">{errors.email}</p>
              )}
            </div>
            <div className="space-y-2">
              <Label htmlFor="password">密码</Label>
              <Input
                id="password"
                name="password"
                type="password"
                value={formData.password}
                onChange={handleChange}
                className={errors.password ? 'border-destructive' : ''}
              />
              {errors.password && (
                <p className="text-sm text-destructive">{errors.password}</p>
              )}
            </div>
          </form>
        </CardContent>
        <CardFooter>
          <Button
            type="submit"
            className="w-full"
            disabled={isLoading}
            onClick={handleSubmit}
          >
            {isLoading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            {isLoading ? '初始化中...' : '创建管理员账户'}
          </Button>
        </CardFooter>
      </Card>
    </div>
  );
}