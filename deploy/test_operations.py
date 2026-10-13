import importlib.util
from pathlib import Path
import unittest

spec=importlib.util.spec_from_file_location('operations',Path(__file__).with_name('operations.py'))
ops=importlib.util.module_from_spec(spec)
spec.loader.exec_module(ops)

class OperationsTest(unittest.TestCase):



    def test_release_rejects_mutable_or_injected_images(self):
        for image in ['ghcr.io/org/api:latest','ghcr.io/org/api:abc; touch /tmp/x','../api:abc']:
            with self.assertRaises(ValueError):ops.release_plan(image,'ghcr.io/org/web:'+'a'*40)
        steps=ops.release_plan('ghcr.io/org/api:'+'a'*40,'ghcr.io/org/web:'+'a'*40)
        self.assertLess(steps.index('pull-immutable-images'),steps.index('migrate'))
        self.assertLess(steps.index('migrate'),steps.index('start'))
        self.assertIn('rollback-app-only-on-failed-health-or-smoke',steps)


class ReleaseGateTest(unittest.TestCase):
    def test_successful_release_runs_migrations_and_health_checks(self):
        events=[]
        ops.simulate_release(events.append)
        self.assertEqual(events,['pull-immutable-images','migrate','seed','start','health','https-smoke'])


    def test_migration_failure_never_rolls_database_back(self):
        events=[]
        def action(step):
            events.append(step)
            if step=='migrate':raise RuntimeError('dirty migration')
        with self.assertRaises(RuntimeError):ops.simulate_release(action)
        self.assertNotIn('start',events)
        self.assertNotIn('rollback-app',events)

    def test_failed_application_start_rolls_application_back(self):
        events=[]
        def action(step):
            events.append(step)
            if step=='start':raise RuntimeError('image cannot start')
        with self.assertRaises(RuntimeError):ops.simulate_release(action)
        self.assertEqual(events[-2:],['rollback-app','rollback-health'])
        self.assertNotIn('rollback-database',events)

    def test_failed_smoke_rolls_only_application_back(self):
        events=[]
        def action(step):
            events.append(step)
            if step=='https-smoke':raise RuntimeError('HTTP 500')
        with self.assertRaises(RuntimeError):ops.simulate_release(action)
        self.assertEqual(events[-2:],['rollback-app','rollback-health'])
        self.assertNotIn('rollback-database',events)




class PersistenceTest(unittest.TestCase):
    def test_read_only_verification_rejects_lost_state_before_writing(self):
        snapshot={'slug':'goroutines','note_id':'saved-note','note_body':'saved body','checklist_id':'saved-item','status':'mastered'}
        for lost in ('status','bookmarked','review_required','checklist','note'):
            with self.subTest(lost=lost):
                calls=[]
                state={'status':'mastered','bookmarked':True,'review_required':True,'checklist':{'saved-item':True}}
                notes=[{'id':'saved-note','body_markdown':'saved body'}]
                if lost=='note':notes=[]
                elif lost=='checklist':state['checklist']={}
                elif lost=='status':state['status']='not_started'
                else:state[lost]=False
                def request(path):
                    calls.append(path)
                    return {'data':notes} if path=='/notes' else {'data':{'user_state':state}}
                with self.assertRaises(ValueError):ops.verify_snapshot(request,snapshot)
                self.assertTrue(all(path.startswith('/topics/') or path=='/notes' for path in calls))


class SSHPolicyTest(unittest.TestCase):
    def test_dry_run_ssh_command_enforces_dedicated_user_and_host_verification(self):
        for user in ('root','deploy;uname','-oProxyCommand=x'):
            with self.assertRaises(ValueError):ops.ssh_command('server.example',user,'/private/key','/private/known_hosts')
        command=ops.ssh_command('server.example','tracker-deploy','/private/key','/private/known_hosts')
        self.assertIn('StrictHostKeyChecking=yes',command)
        self.assertIn('ForwardAgent=no',command)
        self.assertIn('IdentitiesOnly=yes',command)
        self.assertIn('BatchMode=yes',command)
        self.assertEqual(command[-1],'tracker-deploy@server.example')

if __name__=='__main__':unittest.main()
