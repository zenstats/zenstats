import 'flag-icons/css/flag-icons.min.css';
import { useState, useEffect, useMemo } from 'react';

import { COUNTRY_LIST } from '@/constants/countries';
import { useParams } from 'react-router-dom';
import { Separator } from "@components/ui/separator"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";

import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import SearchSelect from "@components/search-select";

import axios, { type BaseResponse } from "@utils/axios";
import { AxiosError } from "axios";
import { toast } from "sonner";

// Define interface for Country shield rule
interface ShieldCountryRule {
  id: number;
  site_id: number;
  country_code: string;
  country_name: string;
  action: string;
  description: string;
  added_by: string;
  created_at: string;
  updated_at: string;
}

// API client for country shield management
const fetchCountries = async (domain: string): Promise<ShieldCountryRule[]> => {
  try {
    const response = await axios.get<BaseResponse<ShieldCountryRule[]>>(`/sites/${domain}/shield/country`);
    return response.data.data || [];
  } catch (error) {
    console.error('Failed to fetch countries list:', error);
    throw new Error(error instanceof AxiosError ? error.response?.data.error || 'Failed to fetch countries list' : 'Failed to fetch countries list');
  }
};

const addCountry = async (domain: string, countryCode: string, description: string): Promise<ShieldCountryRule> => {
  try {
    const response = await axios.post<BaseResponse<ShieldCountryRule>>(`/sites/${domain}/shield/country`, {
      country_code: countryCode,
      description,
      action: 'deny'
    });
    // Check business code in response
    if (response.data.code !== 200) {
      throw new Error(`${response.data.message || 'Operation failed'}: ${response.data.error || ''}`);
    }
    return response.data.data as ShieldCountryRule;
  } catch (error) {
     console.error('Failed to add hostname:', error);
    let errorMsg = 'Failed to add hostname';
    let errorDetail = '';
    if (error instanceof AxiosError) {
      const responseData = error.response?.data || {};
      errorMsg = responseData.message || errorMsg;
      errorDetail = responseData.error || '';
    } else if (error instanceof Error) {
      console.log('error', error)
      errorMsg = error.message || errorMsg;
    }
    if (errorDetail.includes('duplicate key value violates unique constraint')) {
      errorMsg = 'Hostname rule already exists';
      errorDetail = 'A rule with this hostname pattern already exists for this site';
    }
    console.log(errorMsg, errorDetail)
    throw new Error(`${errorMsg}: ${errorDetail}`.trim());
  }
};

interface Params extends Record<string, string | undefined> {
  domain?: string;
}

export default function SettingShieldsCountries() {
  const [countryList, setCountryList] = useState<ShieldCountryRule[]>([]);
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [newCountryCode, setNewCountryCode] = useState('');
  const [newCountryDescription, setNewCountryDescription] = useState('');
  const countryOptions = useMemo(() => COUNTRY_LIST.map(country => ({
      value: country.code,
      label: <><span className={`fi fi-${country.code.toLowerCase()} mr-2`}></span>{country.name} ({country.code})</>,
      searchLabel: `${country.name} (${country.code})`
    })), []);
  const [loading, setLoading] = useState(false);
  const [deletingId, setDeletingId] = useState<number | null>(null);
  const { domain: domainParam } = useParams<Params>();
  const domain = domainParam!;

  // Handle country deletion
  const handleDeleteCountry = async (id: number) => {
    if (!domain || !window.confirm('Are you sure you want to unblock this country?')) return;
    setLoading(true);
    setDeletingId(id);
    try {
      await axios.delete(`/sites/${domain}/shield/country/${id}`);
      // Refresh country list
      const updatedCountries = await fetchCountries(domain);
      setCountryList(updatedCountries);
      toast.success('Country unblocked successfully');
    } catch (error) {
      console.error('Error deleting country:', error);
      if (error instanceof AxiosError) {
        const status = error.response?.status;
        const errorData = error.response?.data;
        toast.error(`Failed to unblock country (${status || 'Unknown'})`, {
          description: errorData?.message || errorData?.error || 'Unknown error'
        });
      } else {
        toast.error('Failed to unblock country', {
          description: 'Unknown error'
        });
      }
    } finally {
      setLoading(false);
      setDeletingId(null);
    }
  };

  // Load country list on component mount and domain change
  useEffect(() => {
    if (!domain) return;
    const loadCountries = async () => {
      setLoading(true);
      try {
        const countries = await fetchCountries(domain);
        setCountryList(countries);
      } catch (error) {
        console.error('Error loading country list:', error);
        toast.error('加载国家/地区列表失败', {
          description: error instanceof AxiosError ? error.response?.data.error : '未知错误'
        });
      } finally {
        setLoading(false);
      }
    };
    loadCountries();
  }, [domain]);

  // Handle country addition
  const handleAddCountry = async () => {
    if (!newCountryCode.trim() || !domain) return;
    setLoading(true);
    try {
      await addCountry(domain, newCountryCode.trim().toUpperCase(), newCountryDescription.trim());
      // Refresh country list
      const updatedCountries = await fetchCountries(domain);
      setCountryList(updatedCountries);
      // Show success toast and reset form
      setNewCountryCode('');
      setNewCountryDescription('');
      setIsModalOpen(false);
      toast.success('Country blocked successfully');
    } catch (error) {
      console.error('Error adding country:', error);
      const errorMessage = error instanceof Error ? error.message : 'Failed to add country';
      toast.error('添加国家/地区失败', {
        description: errorMessage
      });
    } finally {
      setLoading(false);
    }
  };

  // Validate domain exists
  if (!domainParam) {
    return <div>Loading...</div>;
  }

  const getCountryName = (code: string) => {
    const country = COUNTRY_LIST.find(c => c.code === code);
    return country ? country.name : code;
  };

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <div>
            <CardTitle>Country Blacklist</CardTitle>
            <CardDescription>Block incoming traffic from specific countries/regions (blacklist).</CardDescription>
          </div>
          <Button onClick={() => {
            setIsModalOpen(true);
          }}>Add Blocked Country</Button>
        </CardHeader>
        <CardContent>
          <Separator className="my-6" />

          {loading ? (
            <div className="text-center py-8">Loading countries...</div>
          ) : countryList.length === 0 ? (
            <div className="text-center py-8 text-gray-500">No countries in blacklist yet</div>
          ) : (
            <div className="space-y-4">
              {countryList.map((country) => (
                <div key={country.id} className="flex items-center justify-between p-4 border rounded-lg">
                  <div>
                    <p className="font-medium">
                      <span className={`fi fi-${country.country_code.toLowerCase()} mr-2`}></span>{getCountryName(country.country_code)} ({country.country_code})
                    </p>
                    {country.description && (
                      <p className="text-sm text-gray-500 mt-1">{country.description}</p>
                    )}
                    <p className="text-xs text-gray-400">Added by {country.added_by} on {new Date(country.created_at).toLocaleString()}</p>
                  </div>
                  <button
                    onClick={() => handleDeleteCountry(country.id)}
                    className="text-red-500 hover:text-red-700"
                    disabled={loading}
                  >
                    {loading && country.id === deletingId ? 'Deleting...' : 'Delete'}
                  </button>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      <Dialog open={isModalOpen} onOpenChange={setIsModalOpen}>
        <DialogContent className="sm:max-w-[425px] max-h-[80vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>Add Country to Blacklist</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-4">
            <div className="space-y-2">
              <Label htmlFor="country-code">Country Code</Label>
              <SearchSelect
                className="w-full"
                options={countryOptions}
                value={newCountryCode}
                onValueChange={setNewCountryCode}
                placeholder="Select a country"
                searchPlaceholder="Search country..."
                disabled={loading}
              />
              <p className="text-xs text-gray-500">Use ISO 3166-1 alpha-2 country codes (2 uppercase letters)</p>
            </div>
            <div className="space-y-2">
              <Label htmlFor="country-description">Description (optional)</Label>
              <Textarea
                id="country-description"
                value={newCountryDescription}
                onChange={(e) => setNewCountryDescription(e.target.value)}
                placeholder="Why is this country blocked?"
                className="resize-none"
                rows={3}
                disabled={loading}
              />
            </div>
          </div>
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => setIsModalOpen(false)}
              disabled={loading}
            >
              Cancel
            </Button>
            <Button onClick={handleAddCountry} disabled={loading || !newCountryCode.trim()}>
              {loading ? 'Adding...' : 'Add Country'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}