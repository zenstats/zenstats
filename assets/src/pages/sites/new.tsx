import { Card } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import { Button } from "@/components/ui/button"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { useNavigate } from "react-router-dom"
import { toast } from "sonner"
import { Loader2 } from "lucide-react"
import { useState } from "react"
import { useForm } from "react-hook-form"
import axios from "@utils/axios";
import { AxiosError } from "axios"

type FormValues = {
  domain: string;
  description: string;
  rateLimitUnit: string;
  limit_minute: number;
  rate_seconds?: number;
}

export default function NewSitePage() {
  const navigate = useNavigate();
  const [isLoading, setIsLoading] = useState(false)

  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<FormValues>({
    defaultValues: {
      limit_minute: 1000,
      rateLimitUnit: "second"
    }
  })


  const onSubmit = async (data: FormValues) => {
    setIsLoading(true)
    switch (data.rateLimitUnit) {
      case "minute":
        data.rate_seconds = 60
        break
      case "hour":
        data.rate_seconds = 3600
        break
      default:
        data.rate_seconds = 1
    }

    try {
      axios.post("/sites", data).then(() => {
        setIsLoading(false)
        toast.success("创建成功")
        navigate("/sites")
      }).catch((err) => {
        toast.error("创建失败", {
          description: err instanceof AxiosError ? err.response?.data.error : "未知错误",
        })
      })

    } catch (err) {
      toast.error("创建失败", {
          description: err instanceof AxiosError ? err.response?.data.error : "未知错误",
        })
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <div className="flex items-center justify-center h-full">
      <Card className="w-full max-w-md p-6 space-y-4">
        <h1 className="text-2xl font-bold text-center">Create New Site</h1>

        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="domain">Site Domain</Label>
            <Input
              {...register("domain", {
                required: "域名不能为空",
                pattern: {
                  value: /^[a-zA-Z0-9][a-zA-Z0-9-]{1,61}[a-zA-Z0-9](?:\.[a-zA-Z]{2,})+$/,
                  message: "域名格式不正确"
                }
              })}
              placeholder="example.com"
              type="text"
              className={errors.domain ? "border-red-500" : ""}
            />
            {errors.domain && <p className="text-sm text-red-500">{errors.domain.message}</p>}
          </div>

          <div className="space-y-2">
            <Label htmlFor="description">Description</Label>
            <Textarea
              {...register("description", {
                maxLength: {
                  value: 200,
                  message: "描述不能超过200个字符"
                }
              })}
              placeholder="Brief description of your site"
              className={errors.description ? "border-red-500" : ""}
            />
            {errors.description && <p className="text-sm text-red-500">{errors.description.message}</p>}
          </div>

          <div className="space-y-2">
            <Label htmlFor="limit_minute">Rate Limit</Label>
            <div className="flex gap-2">
              <Input
                {...register("limit_minute", {
                  valueAsNumber: true,
                  min: 0
                })}
                placeholder="Limit count"
                type="number"
                className={errors.limit_minute ? "border-red-500" : ""}
              />
              <Select
                {...register("rateLimitUnit")}
                defaultValue="second"
              >
                <SelectTrigger className="w-[150px]">
                  <SelectValue placeholder="Unit" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="second">Per Second</SelectItem>
                  <SelectItem value="minute">Per Minute</SelectItem>
                  <SelectItem value="hour">Per Hour</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>

          <div className="flex justify-end gap-2 pt-4">
            <Button type="button" variant="outline" onClick={() => navigate("/sites")}>Cancel</Button>
            <Button type="submit" disabled={isLoading}>
              {isLoading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              {isLoading ? "Submit..." : "Create Site"}
            </Button>
          </div>
        </form>
      </Card>
    </div>
  )
}