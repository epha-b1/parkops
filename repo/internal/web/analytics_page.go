package web

import (
	"context"
	"io"

	"github.com/a-h/templ"
)

func AnalyticsPage(user CurrentUser) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		content := `<section class="space-y-6">
  <div>
    <h1 class="text-3xl font-semibold">Analytics</h1>
    <p class="mt-1 text-slate-600">Occupancy trends, booking distribution, and exception analysis.</p>
  </div>

  <!-- Date range picker -->
  <div class="flex flex-wrap items-end gap-3 rounded-xl border border-slate-200 bg-white p-4 shadow-sm">
    <label class="text-sm">
      <span class="mb-1 block text-slate-600">From</span>
      <input id="aFrom" type="date" class="rounded border border-slate-300 px-3 py-2" />
    </label>
    <label class="text-sm">
      <span class="mb-1 block text-slate-600">To</span>
      <input id="aTo" type="date" class="rounded border border-slate-300 px-3 py-2" />
    </label>
    <button id="loadAnalytics" class="rounded bg-emerald-700 px-4 py-2 text-sm font-medium text-white hover:bg-emerald-800">Load</button>
  </div>

  <!-- Occupancy Trends -->
  <section class="rounded-xl border border-slate-200 bg-white p-5 shadow-sm">
    <h2 class="text-lg font-semibold mb-3">Occupancy Trends</h2>
    <div id="occData" class="space-y-2"></div>
    <div id="occState" class="text-sm text-slate-500 mt-2"></div>
  </section>

  <!-- Booking Distribution -->
  <section class="rounded-xl border border-slate-200 bg-white p-5 shadow-sm">
    <h2 class="text-lg font-semibold mb-3">Booking Distribution</h2>
    <div class="mb-3">
      <select id="pivotBy" class="rounded border border-slate-300 px-3 py-2 text-sm">
        <option value="time">By Time</option>
        <option value="region">By Region</option>
        <option value="category">By Category</option>
        <option value="revenue">By Rate Plan</option>
      </select>
    </div>
    <div id="bookData" class="space-y-1"></div>
    <div id="bookState" class="text-sm text-slate-500 mt-2"></div>
  </section>

  <!-- Exception Trends -->
  <section class="rounded-xl border border-slate-200 bg-white p-5 shadow-sm">
    <h2 class="text-lg font-semibold mb-3">Exception Trends</h2>
    <div id="excData" class="space-y-1"></div>
    <div id="excState" class="text-sm text-slate-500 mt-2"></div>
  </section>

  <!-- Export -->
  <section class="rounded-xl border border-slate-200 bg-white p-5 shadow-sm">
    <h2 class="text-lg font-semibold mb-3">Export Data</h2>
    <div class="flex flex-wrap gap-3 items-end">
      <select id="expScope" class="rounded border border-slate-300 px-3 py-2 text-sm">
        <option value="bookings">Bookings</option>
        <option value="occupancy">Occupancy</option>
        <option value="exceptions">Exceptions</option>
      </select>
      <select id="expFormat" class="rounded border border-slate-300 px-3 py-2 text-sm">
        <option value="csv">CSV</option>
        <option value="excel">Excel</option>
        <option value="pdf">PDF</option>
      </select>
      <button id="exportBtn" class="rounded bg-slate-900 px-4 py-2 text-sm font-medium text-white hover:bg-slate-800">Generate Export</button>
    </div>
    <div id="exportState" class="mt-2 text-sm text-slate-500"></div>
    <div id="exportList" class="mt-3 space-y-1"></div>
  </section>
</section>

<script>
(function(){
  // Set default dates (last 30 days)
  var now = new Date();
  var ago = new Date(now.getTime() - 30*24*3600*1000);
  document.getElementById('aFrom').value = ago.toISOString().slice(0,10);
  document.getElementById('aTo').value = now.toISOString().slice(0,10);

  function toISO(d) { return new Date(d+'T00:00:00Z').toISOString(); }
  function endISO(d) { return new Date(d+'T23:59:59Z').toISOString(); }

  async function loadAll() {
    var from = document.getElementById('aFrom').value;
    var to = document.getElementById('aTo').value;
    if (!from||!to) return;
    loadOccupancy(from,to);
    loadBookings(from,to);
    loadExceptions(from,to);
    loadExports();
  }

  async function loadOccupancy(from,to) {
    var el = document.getElementById('occData');
    var st = document.getElementById('occState');
    st.textContent = 'Loading...';
    try {
      var res = await fetch('/api/analytics/occupancy?from='+toISO(from)+'&to='+endISO(to)+'&granularity=day',{credentials:'same-origin'});
      var data = await res.json();
      if(!res.ok){st.textContent=data.message||'Failed';return;}
      if(!Array.isArray(data)||data.length===0){el.innerHTML='';st.textContent='No data for this period.';return;}
      var maxPct = Math.max.apply(null,data.map(function(d){return d.peak_occupancy_pct||0;}));
      if(maxPct<=0) maxPct=100;
      el.innerHTML = data.map(function(d){
        var pct = d.avg_occupancy_pct||0;
        var barW = Math.max(2,Math.round(pct/maxPct*100));
        var color = pct>=90?'bg-rose-500':pct>=70?'bg-amber-500':'bg-emerald-500';
        var label = '';
        try{label=new Date(d.period).toLocaleDateString();}catch(e){label=d.period;}
        return '<div class="flex items-center gap-3 text-sm">'
          +'<span class="w-24 text-slate-600 shrink-0">'+label+'</span>'
          +'<div class="flex-1 h-5 bg-slate-100 rounded-full overflow-hidden">'
          +'<div class="h-full rounded-full '+color+'" style="width:'+barW+'%"></div></div>'
          +'<span class="w-20 text-right font-medium">'+pct.toFixed(1)+'%</span>'
          +'</div>';
      }).join('');
      st.textContent = data.length+' periods loaded.';
    }catch(e){st.textContent='Error';}
  }

  async function loadBookings(from,to) {
    var el = document.getElementById('bookData');
    var st = document.getElementById('bookState');
    st.textContent = 'Loading...';
    var pivot = document.getElementById('pivotBy').value;
    try {
      var res = await fetch('/api/analytics/bookings?pivot_by='+pivot+'&from='+toISO(from)+'&to='+endISO(to),{credentials:'same-origin'});
      var data = await res.json();
      if(!res.ok){st.textContent=data.message||'Failed';return;}
      if(!Array.isArray(data)||data.length===0){el.innerHTML='';st.textContent='No booking data.';return;}
      var maxCount = Math.max.apply(null,data.map(function(d){return d.count||0;}));
      if(maxCount<=0) maxCount=1;
      el.innerHTML = data.map(function(d){
        var barW = Math.max(2,Math.round((d.count||0)/maxCount*100));
        var label = d.label||'unknown';
        try{if(label.includes('-'))label=new Date(label).toLocaleDateString();}catch(e){}
        return '<div class="flex items-center gap-3 text-sm">'
          +'<span class="w-32 text-slate-600 shrink-0 truncate">'+label+'</span>'
          +'<div class="flex-1 h-5 bg-slate-100 rounded-full overflow-hidden">'
          +'<div class="h-full rounded-full bg-blue-500" style="width:'+barW+'%"></div></div>'
          +'<span class="w-16 text-right font-medium">'+d.count+'</span>'
          +'<span class="w-20 text-right text-slate-500 text-xs">'+d.total_stalls+' stalls</span>'
          +'</div>';
      }).join('');
      st.textContent = data.length+' entries.';
    }catch(e){st.textContent='Error';}
  }

  async function loadExceptions(from,to) {
    var el = document.getElementById('excData');
    var st = document.getElementById('excState');
    st.textContent = 'Loading...';
    try {
      var res = await fetch('/api/analytics/exceptions?from='+toISO(from)+'&to='+endISO(to),{credentials:'same-origin'});
      var data = await res.json();
      if(!res.ok){st.textContent=data.message||'Failed';return;}
      if(!Array.isArray(data)||data.length===0){el.innerHTML='';st.textContent='No exceptions in this period.';return;}
      var total = data.reduce(function(s,d){return s+d.count;},0);
      el.innerHTML = data.map(function(d){
        var pct = total>0?Math.round(d.count/total*100):0;
        var colors = {'gate_stuck':'bg-amber-500','sensor_offline':'bg-rose-500','camera_error':'bg-purple-500'};
        var color = colors[d.exception_type]||'bg-slate-500';
        return '<div class="flex items-center gap-3 text-sm">'
          +'<span class="w-32 text-slate-600 shrink-0">'+d.exception_type+'</span>'
          +'<div class="flex-1 h-5 bg-slate-100 rounded-full overflow-hidden">'
          +'<div class="h-full rounded-full '+color+'" style="width:'+pct+'%"></div></div>'
          +'<span class="w-16 text-right font-medium">'+d.count+'</span>'
          +'</div>';
      }).join('');
      st.textContent = total+' total exceptions.';
    }catch(e){st.textContent='Error';}
  }

  async function loadExports() {
    var el = document.getElementById('exportList');
    try {
      var res = await fetch('/api/exports',{credentials:'same-origin'});
      var data = await res.json();
      if(!res.ok) return;
      var rows = Array.isArray(data)?data:[];
      if(rows.length===0){el.innerHTML='<p class="text-sm text-slate-500">No exports yet.</p>';return;}
      el.innerHTML = rows.slice(0,10).map(function(e){
        var date=''; try{date=new Date(e.created_at).toLocaleString();}catch(x){date=e.created_at;}
        var dl = e.status==='ready'?'<a href="/api/exports/'+e.id+'/download" class="text-emerald-700 underline text-xs">Download</a>':'<span class="text-xs text-slate-400">'+e.status+'</span>';
        return '<div class="flex items-center justify-between rounded border border-slate-200 bg-slate-50 px-3 py-2 text-sm">'
          +'<span>'+e.scope+' ('+e.format+')</span>'
          +'<span class="text-xs text-slate-500">'+date+'</span>'
          +dl+'</div>';
      }).join('');
    }catch(e){}
  }

  document.getElementById('loadAnalytics').onclick = loadAll;
  document.getElementById('pivotBy').onchange = function(){
    var from = document.getElementById('aFrom').value;
    var to = document.getElementById('aTo').value;
    loadBookings(from,to);
  };

  document.getElementById('exportBtn').onclick = async function(){
    var scope = document.getElementById('expScope').value;
    var format = document.getElementById('expFormat').value;
    var st = document.getElementById('exportState');
    st.textContent = 'Generating...';
    try {
      var res = await fetch('/api/exports',{method:'POST',credentials:'same-origin',headers:{'Content-Type':'application/json'},body:JSON.stringify({format:format,scope:scope})});
      if(res.ok||res.status===201){
        st.textContent = 'Export generated!';
        if(window.parkopsToast) window.parkopsToast('Export created','success');
        loadExports();
      } else {
        var err = await res.json().catch(function(){return{};});
        st.textContent = err.message||'Export failed';
        if(window.parkopsToast) window.parkopsToast('Export failed','error');
      }
    }catch(e){st.textContent='Error';}
  };

  loadAll();
})();
</script>`
		return AppLayout(user, "Analytics", "/analytics", content).Render(ctx, w)
	})
}
