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
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { TimeZoneSelector } from "@components/time-zone";
import { useCallback, useEffect, useState } from "react";
import axios, { type BaseResponse } from "@utils/axios";
import type { Site } from "../types/interfaces";
import { useParams } from "react-router-dom";

const settingsGeneralFormSchema = z.object({
  timeZone: z
    .string({
      required_error: "Please select an email to display.",
    }),
});

type SettingsGeneralFormValues = z.infer<typeof settingsGeneralFormSchema>;



export function SettingsGeneralForm() {
  const { domain } = useParams();
  const [site, setSite] = useState<Site | null>(null)
  const form = useForm<SettingsGeneralFormValues>({
    resolver: zodResolver(settingsGeneralFormSchema),
    mode: "onChange",
  });

  // 当 site 变化时，更新表单的 timeZone 字段
  useEffect(() => {
    if (site) {
      console.log(site, 'change timezone')
      form.setValue('timeZone', site.timezone || '');
    }
  }, [site, form]);

  function onSubmitTimeZone(data: SettingsGeneralFormValues) {
    toast("You submitted the following values:", {
      description: (
        <pre className="mt-2 w-[340px] rounded-md bg-slate-950 p-4">
          <code className="text-white">{JSON.stringify(data, null, 2)}</code>
        </pre>
      ),
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
      <Card>
        <CardHeader>
          <CardTitle>Site Timezone</CardTitle>
          <CardDescription>Update your reporting timezone</CardDescription>
        </CardHeader>
        <CardContent>
          <Form {...form}>
            <form onSubmit={form.handleSubmit(onSubmitTimeZone)} className="space-y-8">
              <FormField
                control={form.control}
                name="timeZone"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Reporting Timezone</FormLabel>
                    {/* 将 defaultValue 替换为 value */}
                    <TimeZoneSelector
                      onChange={field.onChange}
                      value={field.value}
                    />
                    <FormMessage />
                  </FormItem>
                )}
              />
              <Button type="submit">Update profile</Button>
            </form>
          </Form>
        </CardContent>
      </Card>
    </div>
  );
}
