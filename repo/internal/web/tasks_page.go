package web

import (
	"context"
	"io"

	"github.com/a-h/templ"
)

func TasksPage(user CurrentUser) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		content := `<section class="space-y-6">
  <div class="flex items-center justify-between">
    <div>
      <h1 class="text-3xl font-semibold">Tasks</h1>
      <p class="mt-1 text-slate-600">View, create and manage campaign tasks.</p>
    </div>
    <button id="refreshTasks" class="rounded border border-slate-300 bg-white px-3 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50 shadow-sm">Refresh</button>
  </div>

  <div id="campaignSelect" class="flex gap-3 items-center text-sm">
    <label class="text-slate-600 font-medium">Campaign:</label>
    <select id="campFilter" class="rounded border border-slate-300 px-3 py-2 text-sm"><option value="">All</option></select>
    <button id="addTaskBtn" class="ml-auto rounded bg-emerald-700 px-4 py-2 text-sm font-medium text-white hover:bg-emerald-800 shadow-sm">+ New Task</button>
  </div>

  <div id="taskState" class="rounded border border-slate-200 bg-slate-50 px-3 py-2 text-sm text-slate-600">Loading...</div>
  <div id="taskList" class="space-y-2"></div>

  <dialog id="taskDialog" class="rounded-xl border border-slate-200 shadow-lg p-6 w-full max-w-md backdrop:bg-black/30">
    <h3 class="text-lg font-semibold mb-4">Create Task</h3>
    <form id="taskForm" class="space-y-3">
      <div>
        <label class="block text-sm font-medium text-slate-700 mb-1">Campaign</label>
        <select id="tfCampaign" required class="w-full rounded border border-slate-300 px-3 py-2 text-sm"></select>
      </div>
      <div>
        <label class="block text-sm font-medium text-slate-700 mb-1">Description</label>
        <textarea id="tfDesc" required class="w-full rounded border border-slate-300 px-3 py-2 text-sm" rows="3"></textarea>
      </div>
      <div class="grid grid-cols-2 gap-3">
        <div>
          <label class="block text-sm font-medium text-slate-700 mb-1">Deadline</label>
          <input type="datetime-local" id="tfDeadline" class="w-full rounded border border-slate-300 px-3 py-2 text-sm">
        </div>
        <div>
          <label class="block text-sm font-medium text-slate-700 mb-1">Reminder (min)</label>
          <input type="number" id="tfReminder" value="60" min="1" class="w-full rounded border border-slate-300 px-3 py-2 text-sm">
        </div>
      </div>
      <div class="flex justify-end gap-2 mt-4">
        <button type="button" id="cancelTask" class="rounded border border-slate-300 px-4 py-2 text-sm text-slate-600 hover:bg-slate-50">Cancel</button>
        <button type="submit" class="rounded bg-emerald-700 px-4 py-2 text-sm font-medium text-white hover:bg-emerald-800">Create</button>
      </div>
    </form>
  </dialog>
</section>

<script>
(function(){
  var campaigns = [];
  var currentCampaign = '';

  async function loadCampaigns() {
    try {
      var res = await fetch('/api/campaigns', {credentials:'same-origin'});
      campaigns = await res.json();
      if (!Array.isArray(campaigns)) campaigns = campaigns.items || [];
      var sel = document.getElementById('campFilter');
      var tf = document.getElementById('tfCampaign');
      sel.innerHTML = '<option value="">All</option>';
      tf.innerHTML = '';
      campaigns.forEach(function(c) {
        sel.innerHTML += '<option value="'+c.id+'">'+esc(c.title)+'</option>';
        tf.innerHTML += '<option value="'+c.id+'">'+esc(c.title)+'</option>';
      });
    } catch(e) {}
  }

  async function loadTasks() {
    document.getElementById('taskState').textContent = 'Loading...';
    document.getElementById('taskList').innerHTML = '';
    try {
      var tasks = [];
      var src = currentCampaign ? [currentCampaign] : campaigns.map(function(c){return c.id;});
      for (var i = 0; i < src.length; i++) {
        var res = await fetch('/api/campaigns/'+src[i]+'/tasks', {credentials:'same-origin'});
        var data = await res.json();
        var items = Array.isArray(data) ? data : (data.items || []);
        items.forEach(function(t){ t._campaign = src[i]; });
        tasks = tasks.concat(items);
      }
      if (tasks.length === 0) { document.getElementById('taskState').textContent = 'No tasks.'; return; }
      document.getElementById('taskState').textContent = tasks.length + ' task(s)';
      document.getElementById('taskList').innerHTML = tasks.map(function(t) {
        var done = !!t.completed_at;
        var cls = done ? 'border-slate-200 bg-white opacity-60' : 'border-emerald-200 bg-emerald-50';
        var badge = done ? '<span class="text-xs text-slate-400">Done</span>' : '<span class="text-xs font-medium text-amber-700 bg-amber-100 px-2 py-0.5 rounded-full">Open</span>';
        var dl = t.deadline ? '<span class="text-xs text-slate-500">Due: '+new Date(t.deadline).toLocaleString()+'</span>' : '';
        var actions = done ? '' : '<button data-complete="'+t.id+'" class="rounded border border-emerald-500 px-2 py-1 text-xs text-emerald-700 hover:bg-emerald-100">Complete</button>';
        return '<div class="rounded-lg border '+cls+' p-4 shadow-sm"><div class="flex items-start justify-between"><div><p class="font-medium text-slate-900">'+esc(t.description)+'</p>'+dl+'</div>'+badge+'</div><div class="mt-2">'+actions+'</div></div>';
      }).join('');
      document.querySelectorAll('[data-complete]').forEach(function(btn){
        btn.onclick = async function(){
          await fetch('/api/tasks/'+btn.getAttribute('data-complete')+'/complete',{method:'POST',credentials:'same-origin'});
          if(window.parkopsToast) window.parkopsToast('Task completed','success');
          loadTasks();
        };
      });
    } catch(e) { document.getElementById('taskState').textContent = 'Error loading tasks'; }
  }

  function esc(s) { var d=document.createElement('div'); d.textContent=s; return d.innerHTML; }

  document.getElementById('campFilter').onchange = function(){ currentCampaign = this.value; loadTasks(); };
  document.getElementById('refreshTasks').onclick = function(){ loadCampaigns().then(loadTasks); };
  document.getElementById('addTaskBtn').onclick = function(){ document.getElementById('taskDialog').showModal(); };
  document.getElementById('cancelTask').onclick = function(){ document.getElementById('taskDialog').close(); };
  document.getElementById('taskForm').onsubmit = async function(e) {
    e.preventDefault();
    var cid = document.getElementById('tfCampaign').value;
    var body = { description: document.getElementById('tfDesc').value, reminder_interval_minutes: parseInt(document.getElementById('tfReminder').value)||60 };
    var dl = document.getElementById('tfDeadline').value;
    if (dl) body.deadline = new Date(dl).toISOString();
    var res = await fetch('/api/campaigns/'+cid+'/tasks',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body),credentials:'same-origin'});
    if (res.ok) { document.getElementById('taskDialog').close(); if(window.parkopsToast) window.parkopsToast('Task created','success'); loadTasks(); }
    else { var d = await res.json(); alert(d.message||'Failed'); }
  };

  loadCampaigns().then(loadTasks);
})();
</script>`
		return AppLayout(user, "Tasks", "/tasks", content).Render(ctx, w)
	})
}
