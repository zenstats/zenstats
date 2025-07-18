import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
import { z } from "zod";

import { toast } from "sonner";
import { Button } from "@components/ui/button";
import {
  Form,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@components/ui/form";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@components/ui/select";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { useCallback, useEffect, useState } from "react";
import axios, { type BaseResponse } from "@utils/axios";
import type { Site } from "../types/interfaces";
import { useParams } from "react-router-dom";
import { Input } from "@components/ui/input";
import timeZones from "@/constants/time-zones.json";
import { Loader2 } from "lucide-react";
import { AxiosError } from "axios";

const timeZoneFormSchema = z.object({
  timeZone: z
    .string({
      required_error: "Please select a timezone.",
    }),
});

const rateLimitFormSchema = z.object({
  limit_minute: z
    .coerce.number({
      required_error: "Please enter a rate limit.",
      invalid_type_error: "Please enter a valid number.",
    })
    .int('Must be an integer')
    .min(0, 'Cannot be negative'),
  rateLimitUnit: z
    .string({
      required_error: "Please select a rate limit unit.",
    }),
});

type TimeZoneFormValues = z.infer<typeof timeZoneFormSchema>;
type RateLimitFormValues = z.infer<typeof rateLimitFormSchema>;

export function SettingsGeneralForm() {
  const [isTimeZoneLoading, setIsTimeZoneLoading] = useState(false)
  const [isRateLimitLoading, setIsRateLimitLoading] = useState(false)
  const [searchQuery, setSearchQuery] = useState("");
  const filteredTimeZones = timeZones.filter((timeZone) =>
    timeZone.label.toLowerCase().includes(searchQuery.toLowerCase()),
  );
  const { domain } = useParams();
  const [site, setSite] = useState<Site | null>(null);
  const timeZoneForm = useForm<TimeZoneFormValues>({
    resolver: zodResolver(timeZoneFormSchema),
    mode: "onChange",
    defaultValues: {
      timeZone: ''
    }
  });

  const rateLimitForm = useForm<RateLimitFormValues>({
    resolver: zodResolver(rateLimitFormSchema),
    mode: "onChange",
    defaultValues: {
      limit_minute: 1000,
      rateLimitUnit: 'second'
    }
  });

  // 当 site 变化时，更新时区表单
  useEffect(() => {
    if (site) {
      timeZoneForm.setValue('timeZone', site.timezone || '');
    }
  }, [site, timeZoneForm]);

  // 当 site 变化时，更新速率限制表单
  useEffect(() => {
    if (site) {
      rateLimitForm.setValue('limit_minute', site.limit_minute || 1000);

      // 根据 rate_seconds 设置速率限制单位
      if (site.rate_seconds === 3600) {
        rateLimitForm.setValue('rateLimitUnit', 'hour');
      } else if (site.rate_seconds === 60) {
        rateLimitForm.setValue('rateLimitUnit', 'minute');
      } else {
        rateLimitForm.setValue('rateLimitUnit', 'second');
      }
    }
  }, [site, rateLimitForm]);

  function onSubmitTimeZone(data: TimeZoneFormValues) {
    // 提交时区修改
    setIsTimeZoneLoading(true);

    axios.put(`/sites/${domain}`, { timezone: data.timeZone }).then(() => {
      setIsTimeZoneLoading(false);
      toast.success("时区修改成功");
    }).catch((err) => {
      setIsTimeZoneLoading(false);
      console.log(err);
      toast.error("时区修改失败", {
        description: err instanceof AxiosError ? err.response?.data.error : "未知错误",
      });
    });
  }

  function onSubmitRateLimit(data: RateLimitFormValues) {
    // 提交速率限制修改
    setIsRateLimitLoading(true);

    // Convert rateLimitUnit to rate_seconds for API
    let rate_seconds = 1; // default to second
    if (data.rateLimitUnit === 'minute') rate_seconds = 60;
    if (data.rateLimitUnit === 'hour') rate_seconds = 3600;

    const submitData = {
      limit_minute: data.limit_minute,
      rate_seconds
    };

    axios.put(`/sites/${domain}`, submitData).then(() => {
      setIsRateLimitLoading(false);
      toast.success("速率限制修改成功");
    }).catch((err) => {
      setIsRateLimitLoading(false);
      console.log(err);
      toast.error("速率限制修改失败", {
        description: err instanceof AxiosError ? err.response?.data.error : "未知错误",
      });
    });
  }

  const fetchSites = useCallback(async () => {
    try {
      const res = await axios.get<BaseResponse<Site>>(`/sites/${domain}`);
      setSite(res.data.data);
    } catch (error) {
      console.error("Failed to fetch sites:", error);
    }
  }, [domain, setSite]);

  useEffect(() => {
    fetchSites();
  }, [fetchSites]);

  return (
    <div className="container mx-auto space-y-6">
      {/* 时区设置表单 */}
      <Form {...timeZoneForm}>
        <form onSubmit={timeZoneForm.handleSubmit(onSubmitTimeZone)} className="space-y-8">
          <Card>
            <CardHeader>
              <CardTitle>时区设置</CardTitle>
              <CardDescription>更新您的报告时区</CardDescription>
            </CardHeader>
            <CardContent>
              <FormField
                control={timeZoneForm.control}
                name="timeZone"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>报告时区</FormLabel>
                    <Select
                      value={field.value}
                      onValueChange={field.onChange}
                    >
                      <SelectTrigger>
                        <SelectValue placeholder="选择时区" />
                      </SelectTrigger>
                      <SelectContent className="relative">
                        <Input
                          value={searchQuery}
                          onChange={(e) => {
                            e.stopPropagation();
                            setSearchQuery(e.target.value);
                          }}
                          placeholder="搜索时区"
                          className="sticky top-0 z-10 bg-white mb-2"
                        />
                        {filteredTimeZones.map((timeZone) => (
                          <SelectItem key={timeZone.value} value={timeZone.value}>
                            {timeZone.label}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <Button type="submit" disabled={isTimeZoneLoading} className="mt-6">
                {isTimeZoneLoading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                {isTimeZoneLoading ? "提交中..." : "更新时区设置"}
              </Button>
            </CardContent>
          </Card>
        </form>
      </Form>

      {/* 速率限制表单 */}
      <Form {...rateLimitForm}>
        <form onSubmit={rateLimitForm.handleSubmit(onSubmitRateLimit)} className="space-y-8">
          <Card>
            <CardHeader>
              <CardTitle>速率限制设置</CardTitle>
              <CardDescription>配置API请求速率限制</CardDescription>
            </CardHeader>
            <CardContent>
              <FormField
                control={rateLimitForm.control}
                name="limit_minute"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>速率限制</FormLabel>
                    <div className="flex gap-2">
                      <Input
                        {...field}
                        type="number"
                        placeholder="限制数量"
                        onChange={(e) => {
                          const value = e.target.value.replace(/[^0-9]/g, '');
                          field.onChange(value);
                        }}
                        className={rateLimitForm.formState.errors.limit_minute ? "border-red-500" : ""}
                      />
                      <Select
                        onValueChange={(value) => rateLimitForm.setValue('rateLimitUnit', value)}
                        value={rateLimitForm.watch('rateLimitUnit')}
                      >
                        <SelectTrigger className="w-[150px]">
                          <SelectValue placeholder="单位" />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="second">每秒</SelectItem>
                          <SelectItem value="minute">每分钟</SelectItem>
                          <SelectItem value="hour">每小时</SelectItem>
                        </SelectContent>
                      </Select>
                    </div>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={rateLimitForm.control}
                name="rateLimitUnit"
                render={({ field }) => (
                  <input type="hidden" {...field} />
                )}
              />
              <Button type="submit" disabled={isRateLimitLoading} className="mt-6">
                {isRateLimitLoading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                {isRateLimitLoading ? "提交中..." : "更新速率限制"}
              </Button>
            </CardContent>
          </Card>
        </form>
      </Form>
    </div>
  );
}